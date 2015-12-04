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
  //"quantum-sicarius.za.net/p2pChat/utils"
  "sync"
  "crypto/md5"
  "time"
  "encoding/hex"
  "github.com/fatih/color"
  "encoding/json"
  //"reflect"
  "github.com/fatih/structs"
  "sort"
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
var data_state string

var nick string

type Node struct {
  ConnectionType string
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

type SyncIndex struct {
  Keys []string
}

type SyncPacket struct {
  Key string
  Value Message
}

type RequestPacket struct {
  Key string
}

type DataTable struct{
  Mutex sync.Mutex
  // Map of chat
  Data_table map[string]Message
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

  //data_table = make(map[string][]string)
  data.Data_table = make(map[string]Message)
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
  // Sync index
  go syncIndex()
  // Connect nodes
  go connectNodes()

  go client(new_node)

  server(host,port)
}

// Get checksum
func (data *DataTable) getDataCheckSum() string {
  data.Mutex.Lock()
  defer data.Mutex.Unlock()

  return Calculate_data_checksum(data.Data_table)
}

func Calculate_data_checksum(table map[string]Message)string {
  mk := make([]string, len(table))
  i := 0
  for k, _ := range table {
    mk[i] = k
    i++
  }
  sort.Strings(mk)

  temp_values := ""
  for _,v := range mk{
    temp_values = temp_values + v
  }

  byte_values := []byte(temp_values)
  md5_sum := md5.Sum(byte_values)

  return hex.EncodeToString(md5_sum[:])
}

func GetIndex(table map[string]Message)[]string {
  var keys []string

  for k,_ := range table {
    keys = append(keys, k)
  }

  return keys
}

// Compares 2 sets of keys and returns an array of keys that are missing
func CompareKeys(table map[string]Message, other []string)[]string {
  var keys []string
  var exists bool

  for _,v := range other {
    exists = false

    for k,_ := range table {
      if (k == v) {
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
func (data *DataTable) writeToTable(message Message){
  data.Mutex.Lock()
  defer data.Mutex.Unlock()


  data.Data_table[message.Key] = message
  //fmt.Println("data updated")
}

// Encode JSON
func Encode_msg(packet Packet) string {
  jsonString, err := json.Marshal(packet)
  if err != nil {
    return "ERROR"
  }
  return string(jsonString[:]) + "\n"
}

// Decode JSON
func Decode_msg(msg string)(Packet, bool){
  var packet Packet
  err := json.Unmarshal([]byte(msg), &packet)
  if err != nil {
    fmt.Println(msg ,err)
    return packet,false
  }
  return packet, true
}

func syncRequest(key string, node Node) {
  message := Encode_msg(Packet{"RequestPacket",structs.Map(RequestPacket{key})})
  unicastMessage(message, node)
}

func syncNode(key string, node Node) {

  msg := data.Data_table[key]

  message := Encode_msg(Packet{"SyncPacket",structs.Map(SyncPacket{key,msg})})
  unicastMessage(message, node)
}

// Send key value pair
func syncIndex() {
  for {
    node := <-toSyncNodes
    message := Encode_msg(Packet{"SyncIndex",structs.Map(SyncIndex{GetIndex(data.Data_table)})})
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

    for k,v := range nodes {
      if v.ConnectionType == "Client" {
        knownHosts = append(knownHosts, k)
      }
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
    //fmt.Println("Got: " ,node.Data)
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
        // This packet is just a keep alive
      } else if packet.Type == "SyncCheck" {
        node.DataChecksum = packet.Data["Checksum"].(string)
        if node.DataChecksum != data_state {
          toSyncNodes <- node
        }
        if packet.Data["KnownHosts"] != nil {
          knownHosts := packet.Data["KnownHosts"].([]interface{})
          var knownHosts_string []string

          //fmt.Println(knownHosts)

          for _,v := range knownHosts {
            knownHosts_string = append(knownHosts_string, v.(string))
          }

          for _,v := range knownHosts_string {
            found := false
            for _,node := range nodes {
              if node.IPAddr == v || node.Connection.LocalAddr().String() == v{
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
          var index_string []string
          for _,v := range index {
            index_string = append(index_string, v.(string))
          }

          missing_keys := CompareKeys(data.Data_table, index_string)
          //fmt.Println(missing_keys)

          for _,v := range missing_keys {
            syncRequest(v, node)
          }
        }

        //go unicastMessage(Encode_msg(), node)
        // If we receive this packet it means the node whishes to get a value of a key
      } else if packet.Type == "RequestPacket" {
        key := packet.Data["Key"].(string)
        syncNode(key, node)
        //fmt.Println(packet)
        // If we receive this packet it means we got data from the other node to populate our table
      } else if packet.Type == "SyncPacket" {
        if packet.Data["Key"] != nil && packet.Data["Value"] != nil{
          //key := packet.Data["Key"].(string)
          var message Message
          //fmt.Println(packet)
          value := packet.Data["Value"].(map[string]interface{})
          for k,v := range value{
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
    nodes[node.IPAddr]=node
  }
}

// ASYNC update checksum
func printCheckSum() {
  data_state = data.getDataCheckSum()
  fmt.Println(data_state)
}

// Message a single node
func unicastMessage(msg string, node Node) {
  _ ,err := node.Connection.Write([]byte(msg))
  if err != nil {
    fmt.Println("Error sending message!", err)
    cleanUpNodesChan <- node
  } else {
    //fmt.Println("Sent to node: ", msg)
  }
}

// Broad cast to all Nodes
func broadCastMessage(msg string) {
  for _,node := range nodes{
    _ ,err := node.Connection.Write([]byte(msg))
    if err != nil {
      fmt.Println("Error sending message!", err)
      cleanUpNodesChan <- node
    } else {
      //fmt.Println("Sent: ", msg)
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
  fmt.Println("Connecting to: ", conn.RemoteAddr())
  go handleConnection("Client",conn)
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
      go handleConnection("Server",conn);
    }
  }
}

func readConnection(node Node) {
  //buf := make([]byte, 4096)
  for {
    //n, err := node.Connection.Read(reader)
    //n, err := reader.ReadLine("\n")
    line, err := bufio.NewReader(node.Connection).ReadBytes('\n')
    if err != nil{
      if err != io.EOF {
        fmt.Printf("Reached EOF")
      }
      cleanUpNodesChan <- node
      break
    }

    node.Data = (string(line))

    inchan <- node
  }
}


func handleConnection(type_of_connection string, conn net.Conn) {
  nodes[conn.RemoteAddr().String()]= Node{type_of_connection,conn, conn.RemoteAddr().String(), "",""}
  readConnection(nodes[conn.RemoteAddr().String()])
}
