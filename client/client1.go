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

func ValidCommand(s string) (string, string, error) {
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
	if strings.Contains(text, "send") {
		strArray := strings.Split(text, " ")
		if len(strArray) > 1 {
			_, param, err := ValidCommand(text)
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
	}
	return text, nil
}

func main() {

	maxSize, err := strconv.Atoi(msjObj.Message("MAX_FILESIZE"))
	if err != nil {
		log.Println(msjObj.Message("HOST_Error_Convert_Max_FileSize"), err)
	}

	conn, _ := net.Dial("tcp", "localhost:4040")

	serverReader := bufio.NewReader(conn)

	for {
		serverResponse, err := serverReader.ReadString('\n')
		if err != nil {
			log.Fatalln(err)
		}
		log.Println(strings.TrimSpace(serverResponse))
		if strings.TrimSpace(serverResponse) == msjObj.Message("HOST_HEAD_Close") {
			break
		}
	}

	for {
		// read in input from stdin
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(msjObj.Message("GUEST_GENERAL_Text_To_Send"))
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Println(err.Error())
			continue
		}

		//Validar send file
		text, err = SendFile(text, maxSize)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		// send to socket
		conn.Write([]byte(string(text)))

		// listen for reply
		message, _ := bufio.NewReader(conn).ReadString('\n')
		message = strings.TrimSuffix(strings.Trim(message, " "), "\n")
		message = strings.TrimSuffix(message, "\r")

		if message == msjObj.Message("HOST_CLIENT_Goodbye") {
			conn.Close()
			break
		}

		fmt.Print(msjObj.Message("GUEST_GENERAL_Mjs_Form_Server") + message + "\n")

		if strings.Contains(message, msjObj.Message("HOST_CLIENT_Client_In_Mode")) &&
			strings.Contains(message, msjObj.Message("HOST_GENERAL_Receive")) {
			for {
				data := make([]byte, maxSize)
				n, err := conn.Read(data)
				if err != nil {
					fmt.Println(msjObj.Message("GUEST_GENERAL_Error_In_Receive_Mode"))
					break
				}
				s := string(data[:n])
				s = strings.TrimSuffix(strings.Trim(s, " "), "\n")
				s = strings.TrimSuffix(s, "\r")

				format := strings.Split(s, "*")[0]
				format = strings.Replace(format, "send", "", 1)
				file := strings.Replace(s, "send"+format+"*", "", 1)
				newUUID := uuid.New()
				dataFile := []byte(string(file))
				filepath := destinyPath + newUUID.String() + "." + format
				ioutil.WriteFile(filepath, dataFile, 0644)
				log.Println(msjObj.Message("GUEST_GENERAL_File_Is_Received") + newUUID.String() + "." + format)
			}
		}
	}
}
