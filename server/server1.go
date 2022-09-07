package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/darm1609/FileServer_Messages_Golang"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Channel struct {
	Name string
}

type Clients struct {
	Client      net.Conn
	ClientName  string
	Channel     Channel
	Mode        string
	Active      bool
	ReceiveFile int
	SendFile    int
}

var clientList = []Clients{}
var channelList = []Channel{}
var messages = FileServer_Messages_Golang.Messages{}
var destinyPath = "box/"

func main() {
	var addr string
	var network string
	var tcpPort = "4040"
	var httpPort = "8080"
	flag.StringVar(&addr, "e", ":"+tcpPort, "service endpoint [ip addr or socket path]")
	flag.StringVar(&network, "n", "tcp", "network protocol [tcp,unix]")
	flag.Parse()

	//Enpoint para API
	mux := mux.NewRouter()
	mux.HandleFunc("/api/FileServer/", ExportStat).Methods("GET", "OPTIONS")

	// Crear listenner HTTP para API
	go http.ListenAndServe(":"+httpPort, mux)

	// Validar protocolos soportados
	switch network {
	case "tcp", "tcp4", "tcp6", "unix":
	default:
		log.Fatalln(messages.Message("HOST_HEAD_Unsoported"), network)
	}

	// Crear listener TCP
	ln, err := net.Listen(network, addr)
	if err != nil {
		log.Fatal(messages.Message("HOST_HEAD_failed_create_listener"), err)
	}

	log.Println(messages.Message("HOST_HEAD_FileServer"))
	log.Printf(messages.Message("HOST_HEAD_ServiceStart")+" (%s) %s\n", network, addr)

	//Crear canales disponibles
	CreateChannels("1", "2")

	// connection-loop - handle incoming requests
	for {
		conn, err := ln.Accept()

		if err != nil {
			fmt.Println(err)
			if err := conn.Close(); err != nil {
				log.Println(messages.Message("HOST_HEAD_failed_close_listener"), err)
			}
			continue
		}

		RegisterConnectedClient(conn)

		log.Println(messages.Message("HOST_HEAD_Connected_to"), conn.RemoteAddr())

		go HandleConnection(conn)

	}
}

func ExportStat(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	json.NewEncoder(rw).Encode(clientList)
}

func RegisterConnectedClient(conn net.Conn) {
	client := Clients{Client: conn, ClientName: conn.RemoteAddr().String(), Active: true}
	clientList = append(clientList, client)
}

func CreateChannels(channels ...string) {
	for _, elem := range channels {
		channel := Channel{Name: elem}
		channelList = append(channelList, channel)
	}
}

func StringArrayToByteArray(str []string) []byte {
	var x []byte
	for i := 0; i < len(str); i++ {
		b := []byte(str[i])
		for j := 0; j < len(b); j++ {
			x = append(x, b[j])
		}
	}
	return x
}

func SendMessageToClient(message string, conn net.Conn) {
	var err error
	msj := []string{message + "\n"}
	_, err = conn.Write(StringArrayToByteArray(msj))
	if err != nil {
		panic(err)
	}
}

func ValidCommand(s string) (string, string, error) {
	strArray := strings.Split(s, " ")
	if strArray[0] == "send" {
		s = strings.Replace(s, "send", "", 1)
		if len(s) == 0 {
			return "", "", errors.New(messages.Message("HOST_CLIENT_Missing_Parameters"))
		}
		return "send", strings.Trim(s, " "), nil
	} else {
		if len(strArray) >= 1 {
			var valid bool = false
			if strArray[0] == "suscribe" || strArray[0] == "mode" || strArray[0] == "q" {
				valid = true
			} else {
				valid = false
			}
			if valid && len(strArray) > 1 {
				return strArray[0], strArray[1], nil
			}
			if valid && len(strArray) == 1 {
				return strArray[0], "", errors.New(messages.Message("HOST_CLIENT_Missing_Parameters"))
			}
		}
	}
	return "", "", errors.New(messages.Message("HOST_CLIENT_Invalid_Syntax"))
}

func ReadCommandAndParam(data []byte, n int) (string, string, error) {
	s := string(data[:n])
	s = strings.Trim(s, " ")
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	command, param, err := ValidCommand(s)
	return command, param, err
}

func ExtractFormatFromParam(param string) (string, string) {
	path := strings.Split(param, "*")[0]
	file := strings.Replace(param, path, "", 1)
	file = strings.Replace(file, "*", "", 1)
	ini := strings.LastIndex(path, "/")
	if ini < 0 {
		ini = 0
	}
	fin := len(path)
	formato := path[ini:fin]
	ini = strings.LastIndex(formato, ".")
	fin = len(formato)
	formato = formato[ini+1 : fin]
	return formato, file
}

func ReceiveFileInServer(param, formato string) error {
	newUUID := uuid.New()
	data := []byte(string(param))
	filepath := destinyPath + newUUID.String() + "." + formato
	err := ioutil.WriteFile(filepath, data, 0644)
	return err
}

func SuscribeClientToChannel(conn net.Conn, channel Channel) {
	for index := range clientList {
		if clientList[index].Client == conn {
			clientList[index].Channel = channel
		}
	}
}

func EstablishClientMode(conn net.Conn, mode string) bool {
	for index := range clientList {
		if clientList[index].Client == conn {
			clientList[index].Mode = mode
			return true
		}
	}
	return false
}

func IsConnectedToAChannel(conn net.Conn) error {
	for index := range clientList {
		if clientList[index].Client == conn && len(clientList[index].Channel.Name) > 0 {
			return nil
		}
	}
	return errors.New(messages.Message("HOST_CLIENT_Must_Subscribe"))
}

func IsInModeSend(conn net.Conn, modeSend string) bool {
	for index := range clientList {
		if clientList[index].Client == conn && clientList[index].Mode == modeSend {
			return true
		}
	}
	return false
}

func ExistClientInModeReceiveOnChannel(conn net.Conn, modeReceive string) error {
	var channel Channel
	for index := range clientList {
		if clientList[index].Client == conn {
			channel = clientList[index].Channel
		}
	}
	for index := range clientList {
		if clientList[index].Channel.Name == channel.Name &&
			clientList[index].Mode == modeReceive &&
			clientList[index].Active {
			return nil
		}
	}
	return errors.New(messages.Message("HOST_CLIENT_No_Client_On_Channel"))
}

func SendFileToAllClientsOnChannel(conn net.Conn, file string, formato string, modeReceive string) bool {
	var channel Channel
	var send bool = false
	for index := range clientList {
		if clientList[index].Client == conn {
			channel = clientList[index].Channel
			clientList[index].SendFile++
		}
	}
	for index := range clientList {
		if clientList[index].Channel.Name == channel.Name &&
			clientList[index].Mode == modeReceive &&
			clientList[index].Active {
			SendMessageToClient("send"+formato+"*"+file+"\n", clientList[index].Client)
			send = true
			clientList[index].ReceiveFile++
		}
	}
	return send
}

func ValidIfSuscribe(conn net.Conn) bool {
	for index := range clientList {
		if len(clientList[index].Channel.Name) > 0 {
			return true
		}
	}
	return false
}

func SetToInactiveClient(conn net.Conn) {
	for index := range clientList {
		if clientList[index].Client == conn {
			clientList[index].Active = false
		}
	}
}

func SuscribeToChannel(conn net.Conn, param string) {
	for _, channel := range channelList {
		if channel.Name == param {
			SendMessageToClient(messages.Message("HOST_CLIENT_Subscribe_To_Channel"), conn)
			SuscribeClientToChannel(conn, channel)
			return
		}
	}
	SendMessageToClient(messages.Message("HOST_CLIENT_Channel_Not_Exist"), conn)
}

func SetMode(modes map[string]string, conn net.Conn, param string) {
	if ValidIfSuscribe(conn) {
		param = strings.ToUpper(param)
		for i, mode := range modes {
			if i == param {
				if EstablishClientMode(conn, i) {
					SendMessageToClient(messages.Message("HOST_CLIENT_Client_In_Mode")+mode, conn)
					return
				}
			}
		}
		SendMessageToClient(messages.Message("HOST_CLIENT_Invalid_Mode"), conn)
		return
	}
	SendMessageToClient(messages.Message("HOST_CLIENT_Must_Subscribe_Before_Mode"), conn)
}

func ProcessFile(conn net.Conn, param string, receiveMode string) {
	var err error

	err = IsConnectedToAChannel(conn)
	if err != nil {
		SendMessageToClient(err.Error(), conn)
		return
	}

	formato, param := ExtractFormatFromParam(param)

	err = ReceiveFileInServer(param, formato)
	if err != nil {
		SendMessageToClient(messages.Message("HOST_CLIENT_Server_Failed_To_Receive"), conn)
		return
	}

	err = ExistClientInModeReceiveOnChannel(conn, receiveMode)
	if err != nil {
		SendMessageToClient(err.Error(), conn)
		return
	}

	if SendFileToAllClientsOnChannel(conn, param, formato, receiveMode) {
		SendMessageToClient(messages.Message("HOST_CLIENT_The_File_Was_Send"), conn)
	} else {
		SendMessageToClient(messages.Message("HOST_CLIENT_Fail_To_Send_File"), conn)
	}
}

func CreateSuscribeInitMessage() string {
	var msj string = "suscribe <"
	for _, channel := range channelList {
		msj = msj + channel.Name + ", "
	}
	msj = msj[:strings.LastIndex(msj, ",")]
	return msj + ">"
}

func SendInfoAndWelcomeMsjToClient(conn net.Conn) error {
	_, err := conn.Write([]byte("Connected...\nUsage:\n" +
		CreateSuscribeInitMessage() +
		"\nmode <S: Send, R: Receive>\nsend filepath\n" +
		messages.Message("HOST_HEAD_Close") + "\n"))
	return err
}

func HandleConnection(conn net.Conn) {
	const sendMode = "S"
	const receiveMode = "R"

	maxSize, err := strconv.Atoi(messages.Message("MAX_FILESIZE"))
	if err != nil {
		log.Println(messages.Message("HOST_Error_Convert_Max_FileSize"), err)
	}

	modes := make(map[string]string)
	modes["S"] = messages.Message("HOST_GENERAL_Send")
	modes["R"] = messages.Message("HOST_GENERAL_Receive")

	err = SendInfoAndWelcomeMsjToClient(conn)

	if err != nil {
		log.Println(messages.Message("HOST_CLIENT_Error_Writing"), err)
		return
	}

	for {
		data := make([]byte, maxSize)
		n, err := conn.Read(data)

		if err != nil {
			SetToInactiveClient(conn)
			conn.Close()
			continue
		}

		command, param, err := ReadCommandAndParam(data, n)

		if command == "q" {
			SendMessageToClient(messages.Message("HOST_CLIENT_Goodbye"), conn)
			SetToInactiveClient(conn)
			conn.Close()
			break
		}

		if err != nil {
			SendMessageToClient(err.Error(), conn)
			continue
		}

		if command == "mode" {
			SetMode(modes, conn, param)
		}

		if command == "suscribe" {
			SuscribeToChannel(conn, param)
		}

		if command == "send" {
			if IsInModeSend(conn, sendMode) {
				ProcessFile(conn, param, receiveMode)
			} else {
				SendMessageToClient(messages.Message("HOST_CLIENT_Must_Set_Mode_Send")+messages.Message("HOST_GENERAL_Send"), conn)
			}
		}
	}
}
