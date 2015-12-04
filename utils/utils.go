package utils

import (
  //"crypto/md5"
  //"encoding/hex"
  //"sync"
  //"strings"
  //"io"
  //"log"
  //"fmt"
  //"sort"
)



func GetIndex(table map[string][]string)[]string {
  var keys []string

  for k,_ := range table {
    keys = append(keys, k)
  }

  return keys
}

// Compares 2 sets of keys and returns an array of keys that are missing
func CompareKeys(table map[string][]string, other []string)[]string {
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
