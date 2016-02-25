package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/fatih/structs"
	"github.com/op/go-logging"
)

var inchan chan Node
var outchan chan string
var toWrite chan string

// Channel to buffer nodes that need closing
var cleanUpNodesChan chan Node
var newNodesChan chan Node
var toSyncNodes chan Node
var toConnectNodes chan string

var nodes map[string]Node
var data DataTable

// Current revision of chat
// Calculated by the hash of all values in map
var dataState string

var nick string
var listen string

// Instantiate Logger
var log = logging.MustGetLogger("p2pChat")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

// Node struct
type Node struct {
	ConnectionType string
	Connection     net.Conn
	LocalIP        string
	RemoteIP       string
	Listen         string
	DataChecksum   string
	Data           string
}

// Packet struct
type Packet struct {
	Type string
	Data map[string]interface{}
}

// Message struct
type Message struct {
	Key  string // The key value.
	Time string // Timestamp.
	Nick string // The incoming nickname.
	Data string // The encoded data.
}

// SyncCheck struct
type SyncCheck struct {
	Checksum         string   // The md5 sum of the current datatable of the local node.
	ListeningAddress string   // Current address that this node is listening on.
	KnownHosts       []string // All known hosts that this node knows of.
}

// SyncIndex struct
type SyncIndex struct {
	Keys []string // Array of keys of our table
}

type SyncPacket struct {
	Key   string
	Value Message
}

type RequestPacket struct {
	Key string
}

type DataTable struct {
	Mutex sync.Mutex
	// Map of chat
	DataTable map[string]Message
}

func init() {
	inchan = make(chan Node)
	outchan = make(chan string)
	toWrite = make(chan string)
	cleanUpNodesChan = make(chan Node)
	newNodesChan = make(chan Node)
	toSyncNodes = make(chan Node)
	toConnectNodes = make(chan string)
	nodes = make(map[string]Node)

	//DataTable = make(map[string][]string)
	data.DataTable = make(map[string]Message)
	//data := new(DataTable{}
	dataState = data.getDataCheckSum()
	fmt.Println("Currect DataChecksum: ", dataState)
}

func main() {
	// Setup Logging
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backend2Formatter := logging.NewBackendFormatter(backend2, format)

	// Only errors and more severe messages should be sent to backend1
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.ERROR, "")

	// Set the backends to be used.
	logging.SetBackend(backend1Leveled, backend2Formatter)

	var host string
	var port string
	var newNode string

	fmt.Printf("Enter host (Leave blank for localhost): ")
	fmt.Scanln(&host)

	fmt.Printf("Enter port (Leave blank for 8080): ")
	fmt.Scanln(&port)

	fmt.Print("Enter a nick name:")
	fmt.Scanln(&nick)

	fmt.Print("Enter another node's address:")
	fmt.Scanln(&newNode)

	if len(host) == 0 || host == "" {
		host = "localhost"
	}

	if len(port) == 0 || port == "" {
		port = "8080"
	}

	if len(nick) == 0 || nick == "" {
		nick = "IsItSoHardToGetANick?"
	}

	// Linten to user keystrokes
	go clientInput()
	// Start printing routine
	go handleIncoming()
	// Handle cleanup
	go cleanUpNodes()
	// Sync keep alive
	go syncCheck()
	// Sync index
	go syncIndex()
	// Connect nodes
	go connectNodes()

	go client(newNode)

	server(host, port)
}

// Get checksum
func (data *DataTable) getDataCheckSum() string {
	data.Mutex.Lock()
	defer data.Mutex.Unlock()

	return calculateDataChecksum(data.DataTable)
}

// TODO
func calculateDataChecksum(table map[string]Message) string {
	mk := make([]string, len(table))
	i := 0
	for k := range table {
		mk[i] = k
		i++
	}
	sort.Strings(mk)

	tempValues := ""
	for _, v := range mk {
		tempValues = tempValues + v
	}

	byteValues := []byte(tempValues)
	md5Sum := md5.Sum(byteValues)

	return hex.EncodeToString(md5Sum[:])
}

func getIndex(table map[string]Message) []string {
	var keys []string

	for k := range table {
		keys = append(keys, k)
	}

	return keys
}

// Compares 2 sets of keys and returns an array of keys that are missing
func compareKeys(table map[string]Message, other []string) []string {
	var keys []string
	var exists bool

	for _, v := range other {
		exists = false

		for k := range table {
			if k == v {
				exists = true
				//fmt.Println("Exists")
				break
			}
		}

		if exists != true {
			//fmt.Println("Does not exist")
			keys = append(keys, v)
		}
	}

	return keys
}

// Write to table
func (data *DataTable) writeToTable(message Message) {
	data.Mutex.Lock()
	defer data.Mutex.Unlock()

	data.DataTable[message.Key] = message
}

// Encode JSON
func encodeMsg(packet Packet) string {
	jsonString, err := json.Marshal(packet)
	if err != nil {
		return "ERROR"
	}
	return string(jsonString[:]) + "\n"
}

// Decode JSON
func decodeMsg(msg string) (Packet, bool) {
	var packet Packet
	err := json.Unmarshal([]byte(msg), &packet)
	if err != nil {
		fmt.Println(msg, err)
		return packet, false
	}
	return packet, true
}

func syncRequest(key string, node Node) {
	message := encodeMsg(Packet{"RequestPacket", structs.Map(RequestPacket{key})})
	unicastMessage(message, node)
}

func syncNode(key string, node Node) {

	msg := data.DataTable[key]

	message := encodeMsg(Packet{"SyncPacket", structs.Map(SyncPacket{key, msg})})
	unicastMessage(message, node)
}

// Send key value pair
func syncIndex() {
	for {
		node := <-toSyncNodes
		message := encodeMsg(Packet{"SyncIndex", structs.Map(SyncIndex{getIndex(data.DataTable)})})
		unicastMessage(message, node)
	}
}

func connectNodes() {
	for {
		host := <-toConnectNodes
		client(host)
	}
}

// Broadcast current checksum and known hosts
func syncCheck() {
	for {
		var knownHosts []string

		for _, v := range nodes {
			if v.Listen != "nill" {
				knownHosts = append(knownHosts, v.Listen)
			}
		}
		broadCastMessage(encodeMsg(Packet{"SyncCheck", structs.Map(SyncCheck{dataState, listen, knownHosts})}))
		time.Sleep(time.Second * 5)
	}
}

func printReply(message Message) {
	//node := data.DataTable[key]

	timestamp, _ := time.Parse(time.RFC1123, message.Time)
	//fmt.Println(timestamp)
	localTime := timestamp.Local().Format(time.Kitchen)
	//fmt.Println(node[0], node[1], node[2])
	color.Set(color.FgYellow)
	fmt.Printf("<%s> ", localTime)
	color.Set(color.FgGreen)
	fmt.Printf("%s: ", message.Nick)
	color.Set(color.FgCyan)
	fmt.Printf(message.Data)
	color.Unset()
}

// Display incoming messages
func handleIncoming() {
	for {
		node := <-inchan
		//fmt.Println("Got: " ,node.Data)
		packet, success := decodeMsg(node.Data)
		//fmt.Println(key,value)
		if success {
			//printReply(packet)
			//fmt.Println(packet)
			if packet.Type == "Message" {
				//message := packet.Data
				//fmt.Println(message)
				key := packet.Data["Key"].(string)
				timeStamp := packet.Data["Time"].(string)
				nickname := packet.Data["Nick"].(string)
				dataPacket := packet.Data["Data"].(string)
				message := Message{key, timeStamp, nickname, dataPacket}
				data.writeToTable(message)
				go printCheckSum()
				printReply(message)
				// This packet is just a keep alive
			} else if packet.Type == "SyncCheck" {
				node.DataChecksum = packet.Data["Checksum"].(string)
				if node.DataChecksum != dataState {
					toSyncNodes <- node
				}

				if packet.Data["ListeningAddress"] != nil {
					node.Listen = packet.Data["ListeningAddress"].(string)
					nodes[node.RemoteIP] = node
				}

				if packet.Data["KnownHosts"] != nil {
					knownHosts := packet.Data["KnownHosts"].([]interface{})
					var knownHostsString []string

					log.Debug(packet)

					for _, v := range knownHosts {
						knownHostsString = append(knownHostsString, v.(string))
					}

					for _, v := range knownHostsString {
						found := false
						for _, node := range nodes {
							if node.RemoteIP == v || node.LocalIP == v || node.Listen == v || listen == v || node.Listen == "nill" {
								found = true
								break
							}
						}

						if found != true {
							toConnectNodes <- v
						}
					}

				}
				// If we receive this packet it means there is a mismatch with the data and we need to correct it
			} else if packet.Type == "SyncIndex" {
				if packet.Data["Keys"] != nil {
					index := packet.Data["Keys"].([]interface{})
					var indexString []string
					for _, v := range index {
						indexString = append(indexString, v.(string))
					}

					missingKeys := compareKeys(data.DataTable, indexString)
					//fmt.Println(missingKeys)

					for _, v := range missingKeys {
						syncRequest(v, node)
					}
				}

				//go unicastMessage(encodeMsg(), node)
				// If we receive this packet it means the node whishes to get a value of a key
			} else if packet.Type == "RequestPacket" {
				key := packet.Data["Key"].(string)
				syncNode(key, node)
				//fmt.Println(packet)
				// If we receive this packet it means we got data from the other node to populate our table
			} else if packet.Type == "SyncPacket" {
				if packet.Data["Key"] != nil && packet.Data["Value"] != nil {
					//key := packet.Data["Key"].(string)
					var message Message

					value := packet.Data["Value"].(map[string]interface{})
					for k, v := range value {
						if k == "Data" {
							message.Data = v.(string)
						} else if k == "Key" {
							message.Key = v.(string)
						} else if k == "Time" {
							message.Time = v.(string)
						} else if k == "Nick" {
							message.Nick = v.(string)
						}
					}

					data.writeToTable(message)
					printReply(message)
					go printCheckSum()
				}
			}
		}
		nodes[node.RemoteIP] = node
	}
}

// ASYNC update checksum
func printCheckSum() {
	dataState = data.getDataCheckSum()
	log.Info(dataState)
}

// Message a single node
func unicastMessage(msg string, node Node) {
	_, err := node.Connection.Write([]byte(msg))
	if err != nil {
		log.Error("Error sending message!", err)
		cleanUpNodesChan <- node
	} else {
		//fmt.Println("Sent to node: ", msg)
	}
}

// Broad cast to all Nodes
func broadCastMessage(msg string) {
	for _, node := range nodes {
		_, err := node.Connection.Write([]byte(msg))
		if err != nil {
			log.Error("Error sending message!", err)
			cleanUpNodesChan <- node
		} else {
			//fmt.Println("Sent: ", msg)
		}
	}
}

func cleanUpNodes() {
	for {
		node := <-cleanUpNodesChan
		log.Info("Cleaning up Connection: " + node.RemoteIP)
		err := node.Connection.Close()
		if err != nil {
			log.Error("Failed to close!", err)
		}
		delete(nodes, node.RemoteIP)
	}
}

func processInput(msg string) {

	timestamp := time.Now().Format(time.RFC1123)
	md5SumKey := md5.Sum([]byte(timestamp + msg))
	key := hex.EncodeToString(md5SumKey[:])
	//msg_array := []string{key,timestamp,nick,msg}

	//data_map := make(map[string][]string)
	//data_map["Message"] = msg_array
	message := Message{key, timestamp, nick, msg}

	input := encodeMsg(Packet{"Message", structs.Map(message)})
	//fmt.Println(input)

	go broadCastMessage(input)

	data.writeToTable(message)
	go printCheckSum()

}

// Handle client input
func clientInput() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// Sleep to prevent duplicates
		time.Sleep(time.Millisecond)

		input := scanner.Text()
		// Check for input
		if input != "" || len(input) > 0 {
			processInput(input)
		}
	}
}

// Client
func client(host string) {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Error("Failed to connect to host!", err)
		return
	}
	fmt.Println("Connecting to: ", conn.RemoteAddr())
	go handleConnection("Client", conn)
}

// Server function
func server(host string, port string) {
	hostString := host + ":" + port

	// Start listening
	ln, err := net.Listen("tcp", hostString)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	log.Info("Listening on: ", ln.Addr())
	listen = ln.Addr().String()

	// Handle connections
	for {
		if conn, err := ln.Accept(); err == nil {
			log.Info("Incomming connection: ", conn.RemoteAddr())
			go handleConnection("Server", conn)
		}
	}
}

func readConnection(node Node) {
	//buf := make([]byte, 4096)
	for {
		//n, err := node.Connection.Read(reader)
		//n, err := reader.ReadLine("\n")
		line, err := bufio.NewReader(node.Connection).ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Warning("Reached EOF")
			}
			cleanUpNodesChan <- node
			break
		}

		node.Data = (string(line))

		inchan <- node
	}
}

func handleConnection(typeOfConnection string, conn net.Conn) {
	nodes[conn.RemoteAddr().String()] = Node{typeOfConnection, conn, conn.LocalAddr().String(), conn.RemoteAddr().String(), "nill", "", ""}
	readConnection(nodes[conn.RemoteAddr().String()])
}
