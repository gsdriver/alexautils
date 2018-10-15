package main

import (
  "fmt"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/dynamodb"
  "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type HandInfo struct {
  Name string`json:"name"`
}

type Item struct {
  Token string`json:"token"`
  Hands []HandInfo`json:"hands"`
}

func main() {
  sess, err := session.NewSession(&aws.Config{
    Region: aws.String("us-east-1")},
  )

  // Create DynamoDB client
  svc := dynamodb.New(sess)

  input := &dynamodb.ScanInput{
    TableName: aws.String("ThreeCardHands"),
  }

  result, err := svc.Scan(input)
  if err != nil {
    fmt.Println(err.Error())
    return
  }

  m := make(map[string]int)
  for _, i := range result.Items {
    item := Item{}
    err = dynamodbattribute.UnmarshalMap(i, &item)
    if err != nil {
      panic(fmt.Sprintf("Couldn't unmarshal record, %v", err))
    }
    for _, v := range item.Hands {
      m[v.Name]++
    }
  }

  fmt.Println(m)
}
