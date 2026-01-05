package impl

import (
	"context"
	"devops/feishu/config"
	"devops/feishu/pkg/robot"
	roboti "devops/feishu/pkg/robot"
	"devops/tools/ioc"
	"devops/tools/logger"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var (
	Logger     = logger.NewLogger("DEBUG")
)

type RoboImpl struct {
	db *gorm.DB
}

func init() {
	ioc.ConController.RegisterContainer(robot.AppName, &RoboImpl{})
}

func NewRoboImpl(db *gorm.DB) *RoboImpl {
	return &RoboImpl{db: db}
}

func (r *RoboImpl) Init() error {
	c, err := config.LoadConfig()
	if err != nil {
		return err
	}
	r.db = c.GetDB()
	if r.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// AutoMigrate
	// Use CreateFeishuRobotRequest as the model because it contains all the fields
	if !r.db.Migrator().HasTable(&robot.CreateFeishuRobotRequest{}) {
		r.db.AutoMigrate(&robot.CreateFeishuRobotRequest{})
	}

	Logger.Info("InitFeishuRobot DB Storage")

	return nil
}

// 查询详情飞书机器人
func (r *RoboImpl) DescribeRobot(ctx context.Context, req *robot.DescribeFeishuRobotRequest) (*robot.FeishuRobot, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	var reqObj robot.CreateFeishuRobotRequest
	if err := r.db.Where("name = ?", req.Name).First(&reqObj).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found
		}
		return nil, err
	}

	return &robot.FeishuRobot{CreateFeishuRobotRequest: &reqObj}, nil
}

// 查询飞书机器人
func (r *RoboImpl) GetRobot(ctx context.Context, req *robot.QueryFeishuRobotRequest) (*robot.FeishuRobotSet, error) {
	var list []*robot.CreateFeishuRobotRequest
	
	tx := r.db.Model(&robot.CreateFeishuRobotRequest{}).Where("del = ?", 0)
	if req.BotName != "" {
		tx = tx.Where("name = ?", req.BotName)
	}

	var total int64
	tx.Count(&total)

	// Pagination
	offset := req.Offset()
	limit := req.PageSize
	if limit == 0 {
		limit = 20
	}
	
	if err := tx.Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}

	set := robot.NewFeishuRobotSet()
	set.Total = total
	for _, item := range list {
		set.Items = append(set.Items, &robot.FeishuRobot{CreateFeishuRobotRequest: item})
	}

	return set, nil
}

// 添加飞书机器人
func (r *RoboImpl) CreateRobot(ctx context.Context, ro *roboti.CreateFeishuRobotRequest) (*roboti.CreateFeishuRobotRequest, error) {
	if ro.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Check if exists
	var count int64
	r.db.Model(&robot.CreateFeishuRobotRequest{}).Where("name = ?", ro.Name).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("robot already exists")
	}

	ro.CreatedAt = time.Now().Unix()
	ro.UpdatedAt = time.Now().Unix()
	ro.Del = 0

	if err := r.db.Create(ro).Error; err != nil {
		return nil, err
	}

	return ro, nil
}

// 更新飞书机器人
func (r *RoboImpl) UpdateRobot(ctx context.Context, ro *roboti.UpdateFeishuRobotRequest) (*roboti.UpdateFeishuRobotRequest, error) {
	if ro.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	var existing robot.CreateFeishuRobotRequest
	if err := r.db.Where("name = ?", ro.Name).First(&existing).Error; err != nil {
		return nil, err
	}

	// Update fields
	if ro.CreateFeishuRobotRequest != nil {
		existing.Webhook = ro.CreateFeishuRobotRequest.Webhook
		existing.Project = ro.CreateFeishuRobotRequest.Project
		existing.AppID = ro.CreateFeishuRobotRequest.AppID
		existing.AppSecret = ro.CreateFeishuRobotRequest.AppSecret
		existing.EncryptKey = ro.CreateFeishuRobotRequest.EncryptKey
		existing.VerificationToken = ro.CreateFeishuRobotRequest.VerificationToken
		existing.UserAccessToken = ro.CreateFeishuRobotRequest.UserAccessToken
		existing.UserRefreshToken = ro.CreateFeishuRobotRequest.UserRefreshToken
		existing.UserTokenExpire = ro.CreateFeishuRobotRequest.UserTokenExpire
	}

	existing.UpdatedAt = time.Now().Unix()

	if err := r.db.Save(&existing).Error; err != nil {
		return nil, err
	}

	return ro, nil
}

// Delete
func (r *RoboImpl) DeleteRobot(ctx context.Context, req *robot.DeleteFeishuRobotRequest) (*robot.DeleteFeishuRobotRequest, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	result := r.db.Where("name = ?", req.Name).Delete(&robot.CreateFeishuRobotRequest{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("robot not found")
	}

	return req, nil
}
