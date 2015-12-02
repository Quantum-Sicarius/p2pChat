package utils

import (
  "crypto/md5"
  "encoding/hex"
  //"sync"
  //"strings"
  //"io"
  //"log"
  //"fmt"
  "sort"
)

func Calculate_data_checksum(table map[string][]string)string {
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

func Update_data_table(msg map[string][]string) {
  //sync.Mutex
}
