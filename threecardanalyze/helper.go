package main

import (
  "fmt"
  "encoding/json"
)

func CreateWinners(dat map[string]int) {
  // Count how many hands there are that beat, tie, and lose to each combination
  // First count the number in each bucket
  m := make(map[int]int)
  for _, v := range dat {
    m[v]++
  }

  index := 0
  total := 0
  for {
    if _, ok := m[index]; !ok {
      break
    }
    total += m[index]
    index++
  }

  // Now iterate through each one to set win and tie
  var n []WinRatio
  index = 0
  loses := 0
  wins := total
  for {
    if _, ok := m[index]; !ok {
      break
    }
    wins -= m[index]
    n = append(n, WinRatio{wins, m[index], loses})
    loses += m[index]
    index++
  }

  // And write this out to JSON
  result, _ := json.Marshal(n)
  fmt.Println(string(result))
}
