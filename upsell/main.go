package main

import (
  "fmt"
  "strings"
  "encoding/json"
  "io/ioutil"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/s3"
)

type Upsell struct {
  Skill string
  Bucket string
  Version string
  Duration float64
  Impression bool
  Triggers float64
  DurationPostImpression float64
}

func (up Upsell) String() string {
  var str string
  str = "Session of length " + fmt.Sprint(up.Duration) + " ms "
  if up.Impression {
    str += "had an impression and the session lasted " + fmt.Sprint(up.DurationPostImpression) + " ms after the impression. "
  } else {
    str += "didn't have an impression. "
  }
  str += "There were " + fmt.Sprint(up.Triggers) + " total triggers. "
  return str
}

const Bucket = "garrett-alexa-upsell"

func main() {
  var slots []Upsell
  var blackjack []Upsell

  sess, _ := session.NewSession(&aws.Config{
    Region: aws.String("us-east-1")},
  )

  // List S3 buckets
  svc := s3.New(sess)
  params := &s3.ListObjectsInput{Bucket: aws.String(Bucket)}
  var keys []string
  svc.ListObjectsPages(params,
    func(page *s3.ListObjectsOutput, lastPage bool) bool {
      for _, obj := range page.Contents {
        keys = append(keys, *obj.Key)
      }
      return true
  })

	// Parallelize over multiple channels
	const NumberOfChannels = 4
	var i int
	var upResult *Upsell
	channels := make([]chan *Upsell, NumberOfChannels)
	for i = 0; i < len(channels); i++ {
		channels[i] = make(chan *Upsell)
	}

  for i = 0; i < len(keys) - (NumberOfChannels - 1); i += NumberOfChannels {
    for j, ch := range channels {
      go readFromS3(ch, svc, keys[i + j])
    }
    for _, ch := range channels {
      upResult = <- ch
      if upResult != nil {
        if upResult.Skill == "slots" {
          slots = append(slots, *upResult)
        } else {
          blackjack = append(blackjack, *upResult)
        }
      }
    }
  }

	// May need to do one more if there were odd number of keys
	for j := 0; j < len(keys) % NumberOfChannels; j++ {
		go readFromS3(channels[0], svc, keys[len(keys) - j - 1])
		upResult = <- channels[0]
		if upResult != nil {
      if upResult.Skill == "slots" {
        slots = append(slots, *upResult)
      } else {
        blackjack = append(blackjack, *upResult)
      }
    }
	}

  SaveToFile("upsell-blackjack.csv", blackjack)
  SaveToFile("upsell-slots.csv", slots)
  fmt.Println(len(blackjack), " blackjack sessions")
  Summarize(blackjack)
  fmt.Println(len(slots), " slot sessions")
  Summarize(slots)
}

func Summarize(ups []Upsell) {
  impLen := 0.0
  impPostLen := 0.0
  impCount := 0
  noimpLen := 0.0
  noimpCount := 0

  for _, item := range ups {
    if item.Impression {
      impCount++
      impLen += item.Duration
      impPostLen += item.DurationPostImpression
    } else {
      noimpCount++
      noimpLen += item.Duration
    }
  }

  fmt.Println(impCount, "sessions with impressions. Average length", impLen / float64(impCount), "post-length", impPostLen / float64(impCount))
  fmt.Println(noimpCount, "sessions with no impressions. Average length", noimpLen / float64(noimpCount))
}

func SaveToFile(filename string, ups []Upsell) {
  text := ""

  for _, up := range ups {
    text += fmt.Sprintf("%s,%s,%f,%f,%t,%f,%s\n", up.Skill, up.Version, up.Duration, up.Triggers, up.Impression, up.DurationPostImpression, up.Bucket)
  }

  ioutil.WriteFile(filename, []byte(text), 0644)
}

func readFromS3(ch chan *Upsell, svc *s3.S3, key string) {
  var up *Upsell

  // Read the contents of the file
  input := &s3.GetObjectInput{
    Bucket: aws.String(Bucket),
    Key: aws.String(key),
  }
  item, err := svc.GetObject(input)
  if err != nil {
    fmt.Println(err)
  } else {
    defer item.Body.Close()
    var data interface{}

    decoder := json.NewDecoder(item.Body)
    if err := decoder.Decode(&data); err != nil {
      // handle error
      fmt.Println(err)
    } else {
      s := strings.Split(key, "/")
      up = readUpsell(data, (s[0] == "slots"))
      if (up != nil) {
        up.Skill = s[0]
      }
    }
  }

  ch <- up
}

func readUpsell(data interface{}, slots bool) *Upsell {
  var up Upsell
  triggers := 0.0

  m := data.(map[string]interface{})
  if (m["end"] != nil) && (m["start"] != nil) {
    up.Duration = m["end"].(float64) - m["start"].(float64)

    if m["bucket"] != nil {
      up.Bucket = m["bucket"].(string)
    }
    if m["version"] != nil {
      up.Version = m["version"].(string)
    } else {
      up.Version = "1.0"
    }

    for _, v := range m {
      switch v.(type) {
        case map[string]interface{}:
          sub := v.(map[string]interface{})
          if sub["impression"] != nil {
            up.Impression = true
            if slots {
              // For slots, the impression is an interface too!
              impress := sub["impression"].(map[string]interface{})
              if impress["time"] != nil {
                up.DurationPostImpression = m["end"].(float64) - impress["time"].(float64)
              }
            } else {
              up.DurationPostImpression = m["end"].(float64) - sub["impression"].(float64)
            }
          }
          if sub["count"] != nil {
            triggers += sub["count"].(float64)
          }
      }
    }

    up.Triggers = triggers
    return &up
  } else {
    return nil
  }
}