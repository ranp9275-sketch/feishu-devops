package robot

import (
	util "devops/tools/middleware"
	"encoding/json"
)

// 通用参数

type Meta struct {
	CreatedAt int64 `json:"created_at" gorm:"column:created_at"`
	Id        int   `json:"id" gorm:"column:id"`
	UpdatedAt int64 `json:"updated_at" gorm:"column:updated_at"`
}

// 飞书机器人信息
type FeishuRobot struct {
	*CreateFeishuRobotRequest
}

type FeishuRobotRequest struct {
	Robot   FeishuRobot `json:"robot" gorm:"column:robot"`
	Project string      `json:"project" gorm:"column:project"`
	Name    string      `json:"name" gorm:"column:name"`
}

type QueryFeishuRobotRequest struct {
	BotName string `json:"bot_name" gorm:"column:bot_name"`

	*util.PageRequest
}

type NewcFeishuRobotRequest struct {
	*FeishuRobotRequest
	*Meta
}

type CreateFeishuRobotRequest struct {
	ID        int    `json:"id" gorm:"column:id"`
	Name      string `json:"name" gorm:"column:name"`
	Webhook   string `json:"webhook" gorm:"column:webhook"`
	Project   string `json:"project" gorm:"column:project"`
	Del       int    `json:"del" gorm:"column:del"`
	CreatedAt int64  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt int64  `json:"updated_at" gorm:"column:updated_at"`
}

func (f *FeishuRobotRequest) TableName() string {
	return "feishu_robot"
}

func NewFeishuRobotRequest() *QueryFeishuRobotRequest {
	return &QueryFeishuRobotRequest{
		PageRequest: util.NewPageRequest(),
	}
}

func NewFeishuRobot() *FeishuRobot {
	return &FeishuRobot{
		CreateFeishuRobotRequest: &CreateFeishuRobotRequest{
			Del: 0,
		},
	}
}

func (r *FeishuRobot) String() string {
	jsonStr, _ := json.Marshal(r)
	return string(jsonStr)
}

type FeishuRobotSet struct {
	Items []*FeishuRobot `json:"Items"`
	Total int64          `json:"total"`
}

func NewFeishuRobotSet() *FeishuRobotSet {
	return &FeishuRobotSet{
		Items: []*FeishuRobot{},
		Total: 0,
	}
}
