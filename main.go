package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

var (
	bot *linebot.Client
	err error
)

func main() {
	bot, err = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Println(err.Error())
		return
	}

	r := gin.Default()
	r.POST("/callback", callbackHandler)
	r.Run(":80")
}

func callbackHandler(c *gin.Context) {
	defer c.Request.Body.Close()
	// 接收請求
	events, err := bot.ParseRequest(c.Request)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		return
	}

	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				quota, err := bot.GetMessageQuota().Do()
				if err != nil {
					log.Println("Quota err:", err)
				}
				// 回覆訊息
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("msg ID: "+message.ID+" Get: "+message.Text+" , \n OK! remain message:"+strconv.FormatInt(quota.Value, 10)+"Header: "+c.Request.Header.Get("X_LINE_SIGNATURE"))).Do(); err != nil {
					log.Println(err.Error())
				}
			}
		}
	}
}
