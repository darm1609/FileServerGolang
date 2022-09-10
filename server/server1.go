package main

import (
	"encoding/json"
	"errors"
	"flag"
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

var clientList = []Clients{}
var channelList = []Channels{}
var messages = FileServer_Messages_Golang.Messages{}
var destinyPath = "box/"
var tcpPort = "4040"
var httpPort = "8080"

type Channels struct {
	Name string
}

func (channel *Channels) CreateChannels(channels ...string) {
	for _, elem := range channels {
		channel := Channels{Name: elem}
		channelList = append(channelList, channel)
	}
}

type Clients struct {
	Client      net.Conn
	ClientName  string
	Channel     Channels
	Mode        string
	Active      bool
	ReceiveFile int
	SendFile    int
}

func (client *Clients) RegisterConnectedClient(conn net.Conn) {
	client.Client = conn
	client.ClientName = conn.RemoteAddr().String()
	client.Active = true
	clientList = append(clientList, *client)
}

func (client *Clients) SendInfoAndWelcomeMsjToClient() error {
	_, err := client.Client.Write([]byte("Connected...\nUsage:\n" +
		CreateSuscribeInitMessage() +
		"\nmode <S: Send, R: Receive>\nsend filepath\n" +
		messages.Message("HOST_HEAD_Close") + "\n"))
	return err
}

func (client *Clients) SetToInactiveClient() {
	for index := range clientList {
		if clientList[index].Client == client.Client {
			clientList[index].Active = false
			client.Client.Close()
			return
		}
	}
}

func (client *Clients) SendMessageToClient(message string) {
	var err error
	msj := []string{message + "\n"}
	_, err = client.Client.Write(StringArrayToByteArray(msj))
	if err != nil {
		panic(err)
	}
}

func (client *Clients) ProcessFile(param string, receiveMode string) {
	var err error

	err = client.IsConnectedToAChannel()
	if err != nil {
		client.SendMessageToClient(err.Error())
		return
	}

	formato, param := ExtractFormatFromParam(param)

	err = ReceiveFileInServer(param, formato)
	if err != nil {
		client.SendMessageToClient(messages.Message("HOST_CLIENT_Server_Failed_To_Receive"))
		return
	}

	err = client.ExistClientInModeReceiveOnChannel(receiveMode)
	if err != nil {
		client.SendMessageToClient(err.Error())
		return
	}

	if client.SendFileToAllClientsOnChannel(param, formato, receiveMode) {
		client.SendMessageToClient(messages.Message("HOST_CLIENT_The_File_Was_Send"))
	} else {
		client.SendMessageToClient(messages.Message("HOST_CLIENT_Fail_To_Send_File"))
	}
}

func (client *Clients) SetMode(modes map[string]string, param string) {
	if client.ValidIfSuscribe() {
		param = strings.ToUpper(param)
		for index, mode := range modes {
			if index == param {
				if client.EstablishClientMode(index) {
					client.SendMessageToClient(messages.Message("HOST_CLIENT_Client_In_Mode") + mode)
					return
				}
			}
		}
		client.SendMessageToClient(messages.Message("HOST_CLIENT_Invalid_Mode"))
		return
	}
	client.SendMessageToClient(messages.Message("HOST_CLIENT_Must_Subscribe_Before_Mode"))
}

func (client *Clients) SuscribeClientToChannel(channel Channels) {
	for index := range clientList {
		if clientList[index].Client == client.Client {
			clientList[index].Channel = channel
		}
	}
}

func (client *Clients) SuscribeToChannel(param string) {
	for _, channel := range channelList {
		if channel.Name == param {
			client.SendMessageToClient(messages.Message("HOST_CLIENT_Subscribe_To_Channel"))
			client.SuscribeClientToChannel(channel)
			return
		}
	}
	client.SendMessageToClient(messages.Message("HOST_CLIENT_Channel_Not_Exist"))
}

func (client *Clients) SendFileToAllClientsOnChannel(file string, formato string, modeReceive string) bool {
	var channel Channels
	var send bool = false
	for index := range clientList {
		if clientList[index].Client == client.Client {
			channel = clientList[index].Channel
			clientList[index].SendFile++
		}
	}
	for index := range clientList {
		if clientList[index].Channel.Name == channel.Name &&
			clientList[index].Mode == modeReceive &&
			clientList[index].Active {
			clientList[index].SendMessageToClient("send" + formato + "*" + file + "\n")
			send = true
			clientList[index].ReceiveFile++
		}
	}
	return send
}

func (client *Clients) EstablishClientMode(mode string) bool {
	for index := range clientList {
		if clientList[index].Client == client.Client {
			clientList[index].Mode = mode
			return true
		}
	}
	return false
}

func (client *Clients) ValidIfSuscribe() bool {
	for index := range clientList {
		if client.Client == clientList[index].Client && len(clientList[index].Channel.Name) > 0 {
			return true
		}
	}
	return false
}

func (client *Clients) IsConnectedToAChannel() error {
	for index := range clientList {
		if clientList[index].Client == client.Client && len(clientList[index].Channel.Name) > 0 {
			return nil
		}
	}
	return errors.New(messages.Message("HOST_CLIENT_Must_Subscribe"))
}

func (client *Clients) IsInModeSend(modeSend string) bool {
	for index := range clientList {
		if clientList[index].Client == client.Client && clientList[index].Mode == modeSend {
			return true
		}
	}
	return false
}

func (client *Clients) ExistClientInModeReceiveOnChannel(modeReceive string) error {
	var channel Channels
	for index := range clientList {
		if clientList[index].Client == client.Client {
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

func ExportStat(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	json.NewEncoder(rw).Encode(clientList)
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

func CreateSuscribeInitMessage() string {
	var msj string = "suscribe <"
	for _, channel := range channelList {
		msj = msj + channel.Name + ", "
	}
	msj = msj[:strings.LastIndex(msj, ",")]
	return msj + ">"
}

func CreteBufferForMessageReceive(client Clients, maxSize int) ([]byte, int, error) {
	data := make([]byte, maxSize)
	n, err := client.Client.Read(data)
	return data, n, err
}

func WaitAndAcceptConnect(listenerTcp net.Listener) (net.Conn, error) {
	conn, err := listenerTcp.Accept()
	return conn, err
}

func HandleConnection(client Clients) {
	const sendMode = "S"
	const receiveMode = "R"

	maxSize, err := strconv.Atoi(messages.Message("MAX_FILESIZE"))
	if err != nil {
		log.Println(messages.Message("HOST_Error_Convert_Max_FileSize"), err)
	}

	modes := make(map[string]string)
	modes[sendMode] = messages.Message("HOST_GENERAL_Send")
	modes[receiveMode] = messages.Message("HOST_GENERAL_Receive")

	err = client.SendInfoAndWelcomeMsjToClient()
	if err != nil {
		log.Println(messages.Message("HOST_CLIENT_Error_Writing"), err)
		return
	}

	for {
		data, n, err := CreteBufferForMessageReceive(client, maxSize)
		if err != nil {
			client.SetToInactiveClient()
			continue
		}

		command, param, err := ReadCommandAndParam(data, n)

		if command == "q" {
			client.SendMessageToClient(messages.Message("HOST_CLIENT_Goodbye"))
			client.SetToInactiveClient()
			break
		}

		if err != nil {
			client.SendMessageToClient(err.Error())
			continue
		}

		if command == "mode" {
			client.SetMode(modes, param)
		}

		if command == "suscribe" {
			client.SuscribeToChannel(param)
		}

		if command == "send" {
			if client.IsInModeSend(sendMode) {
				client.ProcessFile(param, receiveMode)
			} else {
				client.SendMessageToClient(messages.Message("HOST_CLIENT_Must_Set_Mode_Send") + messages.Message("HOST_GENERAL_Send"))
			}
		}
	}
}

func main() {
	var addr string
	var network string

	flag.StringVar(&addr, "e", ":"+tcpPort, "service endpoint [ip addr or socket path]")
	flag.StringVar(&network, "n", "tcp", "network protocol [tcp,unix]")
	flag.Parse()

	//Enpoint para API
	mux := mux.NewRouter()
	mux.HandleFunc("/api/FileServer/", ExportStat).Methods("GET", "OPTIONS")

	// Crear listenner HTTP para API, concurrente
	go http.ListenAndServe(":"+httpPort, mux)

	// Validar protocolos soportados
	switch network {
	case "tcp", "tcp4", "tcp6", "unix":
	default:
		log.Fatalln(messages.Message("HOST_HEAD_Unsoported"), network)
	}

	// Crear listener TCP
	listenerTcp, err := net.Listen(network, addr)
	if err != nil {
		log.Fatal(messages.Message("HOST_HEAD_failed_create_listener"), err)
	}

	log.Println(messages.Message("HOST_HEAD_FileServer"))
	log.Printf(messages.Message("HOST_HEAD_ServiceStart")+" (%s) %s\n", network, addr)

	//Crear canales disponibles
	channel := Channels{}
	channel.CreateChannels("1", "2")

	// connection-loop - handle incoming requests
	for {
		conn, err := WaitAndAcceptConnect(listenerTcp)
		if err != nil {
			log.Println(messages.Message("HOST_HEAD_failed_close_listener"), err)
			continue
		}

		client := Clients{}
		client.RegisterConnectedClient(conn)

		log.Println(messages.Message("HOST_HEAD_Connected_to"), conn.RemoteAddr())

		//Manejador de conexion concurrente
		go HandleConnection(client)

	}
}
