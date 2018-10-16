package main

import (
    "fmt"
    "encoding/json"
    "strings"
    "strconv"
    "time"
    "io/ioutil"
	"sync"
)

type WinRatio struct {
    Wins int
    Ties int
    Loses int
}

// SafeCounter is safe to use concurrently.
type EquivalentSuggestion struct {
	suggestions map[string][]int
	mux sync.Mutex
}

var ranking map[string]int
var winners []WinRatio
var deck []string
var equivalents EquivalentSuggestion
var equivhits int

// Value returns the current value of the counter for the given key.
func (c *EquivalentSuggestion) Value(key string) []int {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.suggestions[key]
}

func (c *EquivalentSuggestion) Put(key string, value []int) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.suggestions[key] = value
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

	// Mapping of equivalent hands to speed things up
	equivalents = EquivalentSuggestion{suggestions: make(map[string][]int)}

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
	result, _ = json.Marshal(equivalents)
	ioutil.WriteFile("equivalents.json", result, 0644)
	fmt.Println(equivhits)
	fmt.Println(elapsed)
}

func analyzehand(ch chan []int, cards string) {
    hand := strings.Split(cards, "-")
    var bestplay []int
    bestodds := 0.0
    var odds []float64
    var bestMapping [][]int

	// First, have we seen an equivalent?  If so, just add it here
	equivalent := equivalentHand(hand)
	if equivalents.Value(equivalent) != nil {
		equivhits++
		bestplay = equivalents.Value(equivalent)
	} else {
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
		// And save this
		equivalents.Put(equivalent, bestplay)
	}

	ch <- bestplay
}

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

func equivalentHand(hand []string) string {
	// Change suits to X, Y, Z
	suit1 := hand[0][len(hand[0]) - 1:]
	suit2 := hand[1][len(hand[1]) - 1:]
	suit3 := hand[2][len(hand[2]) - 1:]

	var newsuit1, newsuit2, newsuit3 string
	newsuit1 = "X"
	if suit1 == suit2 {
		newsuit2 = "X"
	} else {
		newsuit2 = "Y"
	}

	if suit3 == suit1 {
		newsuit3 = newsuit1
	} else if suit3 == suit2 {
		newsuit3 = newsuit2
	} else {
		if newsuit2 == "X" {
			newsuit3 = "Y"
		} else {
			newsuit3 = "Z"
		}
	}

	// Now sort them
	first := strings.Replace(hand[0], suit1, newsuit1, 1)
	second := strings.Replace(hand[1], suit2, newsuit2, 1)
	third := strings.Replace(hand[2], suit3, newsuit3, 1)

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