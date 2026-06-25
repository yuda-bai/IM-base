package models

import (
	"gorm.io/gorm"
)

type Message struct {
	gorm.Model
	FormId   uint   //发送者id
	TargetId uint   //接收者id
	Type     string //消息类型 群聊、私聊、广播
	Media    string //消息媒体类型 1 文本 2 图片 3 音频
	Content  string //消息内容
	Pic      string //图片
	Url      string //链接
	Desc     string //描述
	Amount   int    //其他数字统计
}

func (table *Message) TableName() string {
	return "messages"
}
