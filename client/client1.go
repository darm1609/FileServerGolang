package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/darm1609/FileServer_Messages_Golang"
	"github.com/google/uuid"
)

var msjObj = FileServer_Messages_Golang.Messages{}
var destinyPath = "box/"
var tcpPort = "4040"

func ConnectToServer() net.Conn {
	conn, _ := net.Dial("tcp", "localhost:"+tcpPort)
	return conn
}

func ReadInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(msjObj.Message("GUEST_GENERAL_Text_To_Send"))
	text, err := reader.ReadString('\n')
	return text, err
}

func SendCommandToServerAndObtainRequest(conn net.Conn, command string) string {
	// send to socket
	conn.Write([]byte(string(command)))

	// listen for reply
	message, _ := bufio.NewReader(conn).ReadString('\n')
	message = strings.TrimSuffix(strings.Trim(message, " "), "\n")
	message = strings.TrimSuffix(message, "\r")

	return message
}

func ShowMessageFromServer(message string) {
	fmt.Print(msjObj.Message("GUEST_GENERAL_Mjs_Form_Server") + message + "\n")
}

func ExtratCommand(s string) (string, string, error) {
	strArray := strings.Split(s, " ")
	if len(strArray) >= 1 {
		var valid bool = false
		if strArray[0] == "send" {
			valid = true
		} else {
			valid = false
		}
		if valid && len(strArray) > 1 {
			return strArray[0], strArray[1], nil
		}
		if valid && len(strArray) == 1 {
			return strArray[0], "", errors.New(msjObj.Message("HOST_CLIENT_Missing_Parameters"))
		}
	}
	return "", "", errors.New(msjObj.Message("HOST_CLIENT_Invalid_Syntax"))
}

func AdjustFilePath(filepath string) string {
	filepath = strings.ReplaceAll(filepath, "\\", "/")
	filepath = strings.Trim(filepath, " ")
	filepath = strings.TrimSuffix(filepath, "\n")
	filepath = strings.TrimSuffix(filepath, "\r")
	return filepath
}

func FileToString(paramFilepath string) (string, error) {
	paramFilepath = AdjustFilePath(paramFilepath)

	dat, err := ioutil.ReadFile(paramFilepath)
	if err != nil {
		return "", err
	}

	stringFile := paramFilepath + "*" + string(dat)

	return stringFile, err
}

func ValidFileSize(filepath string, maxSize int) error {
	fi, err := os.Stat(AdjustFilePath(filepath))
	if err != nil {
		return err
	}
	size := fi.Size()
	if size > int64(maxSize) {
		return errors.New(msjObj.Message("GUEST_Error_Max_File_Size") + strconv.Itoa(maxSize) + " Bytes")
	}
	return nil
}

func SendFile(text string, maxSize int) (string, error) {
	strArray := strings.Split(text, " ")
	if len(strArray) > 1 {
		_, param, err := ExtratCommand(text)
		if err != nil {
			return "send", err
		}

		err = ValidFileSize(param, maxSize)
		if err != nil {
			return "send", err
		}

		fileString, err := FileToString(param)
		if err != nil {
			return "send", err
		}

		return "send " + fileString, nil
	}
	return "send", nil
}

func GetInitMessagesFromServer(conn net.Conn) {
	serverReader := bufio.NewReader(conn)
	for {
		serverResponse, err := serverReader.ReadString('\n')
		if err != nil {
			log.Fatalln(err)
		}
		msjFromServer := strings.TrimSpace(serverResponse)
		log.Println(msjFromServer)
		if msjFromServer == msjObj.Message("HOST_HEAD_Close") {
			break
		}
	}
}

func ClientIsInModeReceive(message string) bool {
	if strings.Contains(message, msjObj.Message("HOST_CLIENT_Client_In_Mode")) &&
		strings.Contains(message, msjObj.Message("HOST_GENERAL_Receive")) {
		return true
	}
	return false
}

func WaitAndReadIncomingData(conn net.Conn, maxSize int) ([]byte, int, error) {
	data := make([]byte, maxSize)
	n, err := conn.Read(data)
	return data, n, err
}

func ExtratFileAndFormat(data []byte, n int) (string, string) {
	str := string(data[:n])
	str = strings.TrimSuffix(strings.Trim(str, " "), "\n")
	str = strings.TrimSuffix(str, "\r")
	format := strings.Split(str, "*")[0]
	format = strings.Replace(format, "send", "", 1)
	file := strings.Replace(str, "send"+format+"*", "", 1)
	return format, file
}

func SaveFileReceive(file string, format string) {
	newUUID := uuid.New()
	dataFile := []byte(string(file))
	filepath := destinyPath + newUUID.String() + "." + format
	ioutil.WriteFile(filepath, dataFile, 0644)
	log.Println(msjObj.Message("GUEST_GENERAL_File_Is_Received") + newUUID.String() + "." + format)
}

func ValidCommand(incomingStr string, command string) bool {
	return strings.Contains(incomingStr, command)
}

func main() {
	maxSize, err := strconv.Atoi(msjObj.Message("MAX_FILESIZE"))
	if err != nil {
		log.Println(msjObj.Message("HOST_Error_Convert_Max_FileSize"), err)
	}

	conn := ConnectToServer()

	GetInitMessagesFromServer(conn)

	for {
		command, err := ReadInput()
		if err != nil {
			log.Println(err.Error())
			continue
		}

		if ValidCommand(command, "send") {
			command, err = SendFile(command, maxSize)
			if err != nil {
				log.Println(err.Error())
				continue
			}
		}

		message := SendCommandToServerAndObtainRequest(conn, command)
		if message == msjObj.Message("HOST_CLIENT_Goodbye") {
			conn.Close()
			break
		}

		ShowMessageFromServer(message)

		if ClientIsInModeReceive(message) {
			for {
				data, n, err := WaitAndReadIncomingData(conn, maxSize)
				if err != nil {
					fmt.Println(msjObj.Message("GUEST_GENERAL_Error_In_Receive_Mode"))
					break
				}

				format, file := ExtratFileAndFormat(data, n)

				SaveFileReceive(file, format)
			}
		}
	}
}
