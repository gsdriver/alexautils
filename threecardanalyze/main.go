package main

import (
    "fmt"
    "encoding/json"
    "strings"
    "strconv"
    "time"
    "io/ioutil"
)

type WinRatio struct {
    Wins int
    Ties int
    Loses int
}

var ranking map[string]int
var winners []WinRatio
var deck []string

// For the given set of cards, the opponent's shown card
// and an array of cards to hold, calculates the probability
// of winning your hand
// Dealer up card is only used to exclude that card as a
// possibility for a drawn card - not in determining
// probability of winning (could be a future optimization)
func oddstowin(cards []string, hold int) float64 {
  var total float64
  var evaluated float64

  if (hold == 3) {
    // Holding all three is easy
    total = float64(winners[ranking[handtostring(cards)]].Wins) / 22100.0
    evaluated = 1.0
  } else {
    // Create array of all cards we could be dealt
    // Figure out the odds of winning for each outcome
    // And average the result to provide overall odds of winning
    total = 0.0
    evaluated = 0.0
    var newhand []string
    for i, card := range cards {
      if (i == hold) {
        // Will replace with drawn card
        newhand = append(newhand, "AS")
      }
      newhand = append(newhand, card)
    }
    for _, newcard := range deck {
      iscardnew := true
      for _, card := range cards {
        if (newcard == card) {
          iscardnew = false
          break
        }
      }
      if iscardnew {
        newhand[hold] = newcard
        evaluated += 1.0
        total += oddstowin(newhand, hold + 1)
      }
    }
  }

  // Now average all the odds - we should have assessed 49 different hands
  return (total / evaluated)
}

func analyzehand(ch chan []int, cards string) {
    hand := strings.Split(cards, "-")
    var bestplay []int
    bestodds := 0.0
    var odds []float64
    var bestMapping [][]int

    bestMapping = append(bestMapping, []int{0, 1, 2})
    bestMapping = append(bestMapping, []int{0, 1})
    bestMapping = append(bestMapping, []int{1, 2})
    bestMapping = append(bestMapping, []int{0, 2})
    bestMapping = append(bestMapping, []int{2})
    bestMapping = append(bestMapping, []int{0})
    bestMapping = append(bestMapping, []int{1})
    bestMapping = append(bestMapping, []int{})

    // OK, first hold all
    odds = append(odds, oddstowin(hand, 3))

    // Try holding two cards
    odds = append(odds, oddstowin(hand, 2))
    hand[0], hand[2] = hand[2], hand[0]
    odds = append(odds, oddstowin(hand, 2))
    hand[1], hand[2] = hand[1], hand[2]
    odds = append(odds, oddstowin(hand, 2))

    // Try holding one card (note array is reversed at this point)
    odds = append(odds, oddstowin(hand, 1))
    hand[0], hand[2] = hand[2], hand[0]
    odds = append(odds, oddstowin(hand, 1))
    hand[1], hand[0] = hand[0], hand[1]
    odds = append(odds, oddstowin(hand, 1))

    // Try discarding all
    odds = append(odds, oddstowin(hand, 0))

    // Which one is highest?
    for index, odd := range odds {
        if (odd > bestodds) {
            bestodds = odd
            bestplay = bestMapping[index]
        }
    }
    ch <- bestplay
}

func handtostring(hand []string) string {
  // Assumes a three card hand
  first := hand[0]
  second := hand[1]
  third := hand[2]

  if (first > second) {
    first, second = second, first
  }
  if (second > third) {
    if (first > third) {
      first, second, third = third, first, second
    } else {
      second, third = third, second
    }
  }
  return first + "-" + second + "-" + third
}

func main() {
    // Load the JSON files in
    dat := Ranks()
    err := json.Unmarshal(dat, &ranking)
    if err != nil {
        fmt.Println(err.Error())
        return
    }

    dat = Winners()
    err = json.Unmarshal(dat, &winners)
    if err != nil {
        fmt.Println(err.Error())
        return
    }

    // Set up the deck
    var card string
    suits := []string{"C", "D", "H", "S"}
    for i := 0; i < 13; i++ {
        for j := 0; j < 4; j++ {
            if (i < 9) {
                card = strconv.Itoa(i + 2)
            } else if (i == 9) {
                card = "J"
            } else if (i == 10) {
                card = "Q"
            } else if (i == 11) {
                card = "K"
            } else {
                card = "A"
            }

            card += suits[j]
            deck = append(deck, card)
        }
    }

    // And start analyzing hands
    // Parallelize over mutlple channels
    const NumberOfChannels = 4
    var i int
    var bestplay []int
    var suggestions = make(map[string][]int)
    start := time.Now()
    channels := make([]chan []int, NumberOfChannels)
    for i = 0; i < len(channels); i++ {
        channels[i] = make(chan []int)
    }

    keys := make([]string, len(ranking))
    i = 0
    for k := range ranking {
        keys[i] = k
        i++
    }

    for i = 0; i < len(keys) - (NumberOfChannels - 1); i += NumberOfChannels {
        for j, ch := range channels {
            go analyzehand(ch, keys[i + j])
        }
        for j, ch := range channels {
            bestplay = <- ch
            suggestions[keys[i + j]] = bestplay
        }
    }

    // May need to do one more if there were odd number of keys
    for j := 0; j < len(keys) % NumberOfChannels; j++ {
        go analyzehand(channels[0], keys[len(keys) - j - 1])
        bestplay = <- channels[0]
        suggestions[keys[len(keys) - j - 1]] = bestplay
    }

    t := time.Now()
    elapsed := t.Sub(start)

    result, _ := json.Marshal(suggestions)
    ioutil.WriteFile("suggest.json", result, 0644)
    fmt.Println(elapsed)
}
