package robot

import (
	"context"
	util "devops/tools/middleware"
)

const (
	AppName = "robot"
)

type Service interface {
	CreateRobot(ctx context.Context, req *CreateFeishuRobotRequest) (*CreateFeishuRobotRequest, error)
	GetRobot(ctx context.Context, req *QueryFeishuRobotRequest) (*FeishuRobotSet, error)
	DeleteRobot(ctx context.Context, req *DeleteFeishuRobotRequest) (*DeleteFeishuRobotRequest, error)
	UpdateRobot(ctx context.Context, req *UpdateFeishuRobotRequest) (*UpdateFeishuRobotRequest, error)
	DescribeRobot(ctx context.Context, req *DescribeFeishuRobotRequest) (*FeishuRobot, error)
}

func NewQueryFeishuRobotRequest() *QueryFeishuRobotRequest {
	return &QueryFeishuRobotRequest{
		PageRequest: util.NewPageRequest(),
	}
}

func NewCreateFeishuRobotRequest() *CreateFeishuRobotRequest {
	return &CreateFeishuRobotRequest{}
}

func NewUpdateFeishuRobotRequest(name string) *UpdateFeishuRobotRequest {
	return &UpdateFeishuRobotRequest{
		Name:                     name,
		CreateFeishuRobotRequest: NewCreateFeishuRobotRequest(),
	}
}

func NewDeleteFeishuRobotRequest(name string) *DeleteFeishuRobotRequest {
	return &DeleteFeishuRobotRequest{
		Name: name,
		Del:  1,
	}
}

func NewDescribeFeishuRobotRequest(name string) *DescribeFeishuRobotRequest {
	return &DescribeFeishuRobotRequest{
		Name: name,
	}
}

type DescribeFeishuRobotRequest struct {
	Name string `json:"name" gorm:"column:name"`
}

type UpdateFeishuRobotRequest struct {
	Name string
	Del  int
	*CreateFeishuRobotRequest
}

type DeleteFeishuRobotRequest struct {
	Name string
	Del  int
}
