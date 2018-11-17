package main

import (
  "fmt"
  "strings"
  "encoding/json"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/s3"
)

const Bucket = "garrett-alexa-upsell"

type Upsell struct {
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

func main() {
  sess, err := session.NewSession(&aws.Config{
    Region: aws.String("us-east-1")},
  )

  // Create S3 client
  svc := s3.New(sess)

  result, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: aws.String(Bucket)})
  if err != nil {
    fmt.Println(err)
  }

  var slots []Upsell
  var blackjack []Upsell

  for _, i := range result.Contents {
    // Read the contents of the file
    input := &s3.GetObjectInput{
      Bucket: aws.String(Bucket),
      Key: aws.String(*i.Key),
    }
    fmt.Println("reading ", *i.Key)
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
        s := strings.Split(*i.Key, "/")
        up := readUpsell(data, (s[0] == "slots"))

        if (up != nil) {
          if s[0] == "slots" {
            slots = append(slots, *up)
          } else {
            blackjack = append(blackjack, *up)
          }
          fmt.Println(up)
        }
      }
    }
  }

  fmt.Println(len(blackjack), " blackjack sessions")
  fmt.Println(len(slots), " slot sessions")
}

func readUpsell(data interface{}, slots bool) *Upsell {
  var up Upsell
  triggers := 0.0

  m := data.(map[string]interface{})
  if (m["end"] != nil) && (m["start"] != nil) {
    up.Duration = m["end"].(float64) - m["start"].(float64)

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