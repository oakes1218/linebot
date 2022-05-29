package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

var (
	bot *linebot.Client
	err error
)

var button string = `{
	"type": "template",
	"altText": "This is a buttons template",
	"template": {
		"type": "buttons",
		"thumbnailImageUrl": "https://example.com/bot/images/image.jpg",
		"imageAspectRatio": "rectangle",
		"imageSize": "cover",
		"imageBackgroundColor": "#FFFFFF",
		"title": "Menu",
		"text": "Please select",
		"defaultAction": {
			"type": "uri",
			"label": "View detail",
			"uri": "http://example.com/page/123"
		},
		"actions": [
			{
			  "type": "postback",
			  "label": "Buy",
			  "data": "action=buy&itemid=123"
			},
			{
			  "type": "postback",
			  "label": "Add to cart",
			  "data": "action=add&itemid=123"
			},
			{
			  "type": "uri",
			  "label": "View detail",
			  "uri": "http://example.com/page/123"
			}
		]
	}
  }`

func main() {
	bot, err = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Println(err.Error())
		return
	}

	r := gin.Default()
	r.POST("/callback", callbackHandler)
	r.Run()
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

				userID, err := bot.GetFollowerIDs("").Do()
				if err != nil {
					log.Println(err.Error())
				}

				// 回覆訊息
				if message.Text == "查看活動" {
					leftBtn := linebot.NewMessageAction("left", "left clicked")
					rightBtn := linebot.NewMessageAction("right", "right clicked")
					template := linebot.NewConfirmTemplate("Hello World", leftBtn, rightBtn)
					// template := linebot.NewButtonsTemplate("https://www.facebook.com/win2fitness/photos/a.593850231091748/595671197576318/", "日期", "星期幾", leftBtn, rightBtn)
					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)

					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("msg ID: "+message.ID+" Get: "+message.Text+" , \n OK!"), msg, linebot.NewTextMessage(userID.UserIDs[0])).Do(); err != nil {
						log.Println(err.Error())
					}

				}
			}
		}
	}
}
