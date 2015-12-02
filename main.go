package main

import (
  "net"
  "log"
  "fmt"
  //"strconv"
  "os"
  "bufio"
  //"bytes"
  "io"
  //"io/ioutil"
  "quantum-sicarius.za.net/p2pChat/utils"
  "sync"
  "crypto/md5"
  "time"
  "encoding/hex"
  "github.com/fatih/color"
  "encoding/json"
  //"reflect"
  "github.com/fatih/structs"
)

var inchan chan Node
var outchan chan string
var toWrite chan string
// Channel to buffer nodes that need closing
var cleanUpNodesChan chan Node
var newNodesChan chan Node

var nodes map[string]Node
var data DataTable

// Current revision of chat
// Calculated by the hash of all values in map
var data_state string

var nick string

type Node struct {
  Connection net.Conn
  IPAddr string
  DataChecksum string
  Data string
}

type Packet struct {
  Type string
  Data map[string]interface{}
}

type Message struct {
  Key string
  Time string
  Nick string
  Data string
}

type SyncCheck struct {
  Checksum string
  KnownHosts []string
}

type SyncPacket struct {
  Key string
  Value string
}

type DataTable struct{
  Mutex sync.Mutex
  // Map of chat
  Data_table map[string][]string
}

func init() {
  inchan = make(chan Node)
  outchan = make(chan string)
  toWrite = make(chan string)
  cleanUpNodesChan = make(chan Node)
  newNodesChan = make(chan Node)

  nodes = make(map[string]Node)

  //data_table = make(map[string][]string)
  data.Data_table = make(map[string][]string)
  //data := new(DataTable{}
  data_state = data.getDataCheckSum()
  fmt.Println("Currect DataChecksum: ", data_state)
}

func main() {
  var host string
  var port string
  var new_node string

  fmt.Printf("Enter host (Leave blank for localhost): ")
  fmt.Scanln(&host)

  fmt.Printf("Enter port (Leave blank for 8080): ")
  fmt.Scanln(&port)

  fmt.Print("Enter a nick name:")
  fmt.Scanln(&nick)

  fmt.Print("Enter another node's address:")
  fmt.Scanln(&new_node)

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
  go client(new_node)

  server(host,port)
}

// Get checksum
func (data *DataTable) getDataCheckSum() string {
  data.Mutex.Lock()
  defer data.Mutex.Unlock()

  return utils.Calculate_data_checksum(data.Data_table)
}

// Write to table
func (data *DataTable) writeToTable(message Message){
  data.Mutex.Lock()
  defer data.Mutex.Unlock()


  data.Data_table[message.Key] = []string{message.Time, message.Nick, message.Data}
  //fmt.Println("data updated")
}

// Encode JSON
func Encode_msg(packet Packet) string {
  jsonString, err := json.Marshal(packet)
  if err != nil {
    return "ERROR"
  }
  return string(jsonString[:])
}

// Decode JSON
func Decode_msg(msg string)(Packet, bool){
  var packet Packet
  err := json.Unmarshal([]byte(msg), &packet)
  if err != nil {
    fmt.Println(err)
    return packet,false
  }
  return packet, true
}

func syncCheck() {
  for {
    var knownHosts []string

    for k,_ := range nodes {
      knownHosts = append(knownHosts, k)
    }
    broadCastMessage(Encode_msg(Packet{"SyncCheck",structs.Map(SyncCheck{data_state,knownHosts})}))
    time.Sleep(time.Second * 5)
  }
}

func printReply(message Message) {
  //node := data.Data_table[key]

  timestamp, _ := time.Parse(time.RFC1123, message.Time)
  //fmt.Println(timestamp)
  local_time := timestamp.Local().Format(time.Kitchen)
  //fmt.Println(node[0], node[1], node[2])
  color.Set(color.FgYellow)
  fmt.Printf("<%s> ", local_time)
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
    fmt.Println("Got: " ,node.Data)
    packet,success := Decode_msg(node.Data)
    //fmt.Println(key,value)
    if success {
      //printReply(packet)
      //fmt.Println(packet)
      if packet.Type == "Message" {
        //message := packet.Data
        //fmt.Println(message)
        key := packet.Data["Key"].(string)
        time_stamp := packet.Data["Time"].(string)
        nickname := packet.Data["Nick"].(string)
        data_packet := packet.Data["Data"].(string)
        message := Message{key,time_stamp,nickname,data_packet}
        data.writeToTable(message)
        go printCheckSum()
        printReply(message)
      } else if packet.Type == "SyncCheck"{
        node.DataChecksum = packet.Data["Checksum"].(string)
      }
    }
    nodes[node.IPAddr]=node
  }
}

// ASYNC update checksum
func printCheckSum() {
  data_state = data.getDataCheckSum()
  fmt.Println(data_state)
}

// Broad cast to all Nodes
func broadCastMessage(msg string) {
  for _,node := range nodes{
    _ ,err := node.Connection.Write([]byte(msg))
    if err != nil {
      fmt.Println("Error sending message!", err)
      cleanUpNodesChan <- node
    } else {
      fmt.Println("Sent: ", msg)
    }
  }
}

func cleanUpNodes() {
  for {
    node := <-cleanUpNodesChan
    fmt.Println("Cleaning up Connection: " + node.IPAddr)
    node.Connection.Close()
    delete(nodes, node.IPAddr)
  }
}

func processInput(msg string) {

  timestamp := time.Now().Format(time.RFC1123)
  md5_sum_key := md5.Sum([]byte(timestamp + msg))
  key := hex.EncodeToString(md5_sum_key[:])
  //msg_array := []string{key,timestamp,nick,msg}

  //data_map := make(map[string][]string)
  //data_map["Message"] = msg_array
  message := Message{key,timestamp,nick,msg}

  input := Encode_msg(Packet{"Message",structs.Map(message)})
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
    if (input != "" || len(input) > 0) {
      processInput(input)
    }
  }
}

// Client
func client(host string) {
  conn, err := net.Dial("tcp", host)
  if err != nil {
    fmt.Println("Failed to connect to host!", err)
    return
  }
  go handleConnection(conn)
}

// Server function
func server(host string, port string) {
  host_string := host + ":" + port

  // Start listening
  ln, err := net.Listen("tcp", host_string)
  if err != nil {
    log.Fatalf("Failed to listen:", err)
  }

  fmt.Println("Listening on: ", ln.Addr())

  // Handle connections
  for {
    if conn, err := ln.Accept(); err == nil {
      fmt.Println("Incomming connection: ", conn.RemoteAddr())
      go handleConnection(conn);
    }
  }
}

func readConnection(node Node) {
  buf := make([]byte, 4096)
  for {
    n, err := node.Connection.Read(buf)
    if err != nil || n == 0{
      if err != io.EOF {
        fmt.Printf("Reached EOF")
      }
      cleanUpNodesChan <- node
      break
    }

    node.Data = (string(buf[0:n]))

    inchan <- node
  }
}


func handleConnection(conn net.Conn) {
  nodes[conn.RemoteAddr().String()]= Node{conn, conn.RemoteAddr().String(), "",""}
  readConnection(nodes[conn.RemoteAddr().String()])
}
