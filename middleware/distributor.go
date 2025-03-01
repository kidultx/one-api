package middleware

import (
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/model"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ModelRequest struct {
	Model string `json:"model"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		userGroup, _ := model.CacheGetUserGroup(userId)
		c.Set("group", userGroup)
		var channel *model.Channel
		channelId, ok := c.Get("channelId")
		if ok {
			id, err := strconv.Atoi(channelId.(string))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "无效的渠道 ID",
						"type":    "one_api_error",
					},
				})
				c.Abort()
				return
			}
			channel, err = model.GetChannelById(id, true)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "无效的渠道 ID",
						"type":    "one_api_error",
					},
				})
				c.Abort()
				return
			}
			if channel.Status != common.ChannelStatusEnabled {
				c.JSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"message": "该渠道已被禁用",
						"type":    "one_api_error",
					},
				})
				c.Abort()
				return
			}
		} else {
			// Select a channel for the user
			var modelRequest ModelRequest
			var err error
			if !strings.HasPrefix(c.Request.URL.Path, "/v1/audio") {
				err = common.UnmarshalBodyReusable(c, &modelRequest)
			}
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "无效的请求",
						"type":    "one_api_error",
					},
				})
				c.Abort()
				return
			}
			if strings.HasPrefix(c.Request.URL.Path, "/v1/moderations") {
				if modelRequest.Model == "" {
					modelRequest.Model = "text-moderation-stable"
				}
			}
			if strings.HasSuffix(c.Request.URL.Path, "embeddings") {
				if modelRequest.Model == "" {
					modelRequest.Model = c.Param("model")
				}
			}
			if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
				if modelRequest.Model == "" {
					modelRequest.Model = "dall-e"
				}
			}
			if strings.HasPrefix(c.Request.URL.Path, "/v1/audio") {
				if modelRequest.Model == "" {
					modelRequest.Model = "whisper-1"
				}
			}
			channel, err = model.CacheGetRandomSatisfiedChannel(userGroup, modelRequest.Model)
			if err != nil {
				message := fmt.Sprintf("当前分组 %s 下对于模型 %s 无可用渠道", userGroup, modelRequest.Model)
				if channel != nil {
					common.SysError(fmt.Sprintf("渠道不存在：%d", channel.Id))
					message = "数据库一致性已被破坏，请联系管理员"
				}
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": gin.H{
						"message": message,
						"type":    "one_api_error",
					},
				})
				c.Abort()
				return
			}
		}
		c.Set("channel", channel.Type)
		c.Set("channel_id", channel.Id)
		c.Set("channel_name", channel.Name)
		c.Set("model_mapping", channel.ModelMapping)
		c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channel.Key))
		c.Set("base_url", channel.BaseURL)
		if channel.Type == common.ChannelTypeAzure || channel.Type == common.ChannelTypeXunfei {
			c.Set("api_version", channel.Other)
		}
		c.Next()
	}
}
