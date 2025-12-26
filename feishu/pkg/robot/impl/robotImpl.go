package impl

import (
	"context"
	"devops/feishu/config"
	"devops/tools/ioc"
	"devops/tools/logger"

	"devops/feishu/pkg/robot"
	roboti "devops/feishu/pkg/robot"

	"gorm.io/gorm"
)

func (r *RoboImpl) Init() error {
	cfg, err := config.LoadConfig()
	roboti.NewFeishuRobotSet()

	cfg.LogLevel = "debug"
	logger := logger.NewLogger(cfg.LogLevel)
	logger.Info("InitFeishuRobot")
	if err != nil {
		return err
	}
	//r.db = cfg.GetDB()
	return nil
}

func init() {
	ioc.ConController.RegisterContainer(robot.AppName, &RoboImpl{})
}

type RoboImpl struct {
	db *gorm.DB
}

func NewRoboImpl(db *gorm.DB) *RoboImpl {
	return &RoboImpl{db: db}
}

// 查询详情飞书机器人
func (r *RoboImpl) DescribeRobot(ctx context.Context, req *robot.DescribeFeishuRobotRequest) (*robot.FeishuRobot, error) {
	robot := robot.NewFeishuRobot()

	robot.Name = req.Name

	query := r.db.Model(&roboti.FeishuRobot{}).WithContext(ctx)

	if robot.Name == "" {
		return nil, gorm.ErrInvalidValue
	}
	if err := query.Where("name = ?", robot.Name).First(&robot).Error; err != nil {
		return nil, err
	}

	return robot, nil
}

// 查询飞书机器人
func (r *RoboImpl) GetRobot(ctx context.Context, req *robot.QueryFeishuRobotRequest) (*robot.FeishuRobotSet, error) {
	set := robot.NewFeishuRobotSet()
	query := r.db.Model(&robot.FeishuRobot{}).WithContext(ctx).Select("name").
		Where("del = 0")
	if req.BotName != "" {
		query = query.Where("name = ?", req.BotName)
	}
	if err := query.Count(&set.Total).Error; err != nil {
		return nil, err
	}

	if err := query.Offset(req.Offset()).Limit(req.PageSize).Find(&set.Items).Error; err != nil {
		return nil, err
	}

	return set, nil
}

// 添加飞书机器人
func (r *RoboImpl) CreateRobot(ctx context.Context, ro *roboti.CreateFeishuRobotRequest) (*roboti.CreateFeishuRobotRequest, error) {

	rebot := robot.NewFeishuRobot()

	rebot.CreateFeishuRobotRequest = ro

	if r.db.WithContext(ctx).Save(rebot).Error != nil {
		return nil, r.db.WithContext(ctx).Save(rebot).Error
	}

	return rebot.CreateFeishuRobotRequest, nil
}

// 更新飞书机器人
func (r *RoboImpl) UpdateRobot(ctx context.Context, ro *roboti.UpdateFeishuRobotRequest) (*roboti.UpdateFeishuRobotRequest, error) {
	if ro.Name == "" {
		return nil, gorm.ErrInvalidValue
	}
	robot, err := r.DescribeRobot(ctx, &robot.DescribeFeishuRobotRequest{Name: ro.Name})

	if err != nil {
		return nil, err
	}
	robot.CreateFeishuRobotRequest = ro.CreateFeishuRobotRequest

	if r.db.WithContext(ctx).Save(robot).Error != nil {
		return nil, r.db.WithContext(ctx).Save(robot).Error
	}

	return ro, nil
}

// 删除飞书机器人
func (r *RoboImpl) DeleteRobot(ctx context.Context, ro *roboti.DeleteFeishuRobotRequest) (*roboti.DeleteFeishuRobotRequest, error) {
	robot, err := r.DescribeRobot(ctx, &robot.DescribeFeishuRobotRequest{Name: ro.Name})

	if err != nil {
		return nil, err
	}
	robot.Del = ro.Del

	if r.db.WithContext(ctx).Save(robot).Error != nil {
		return nil, r.db.WithContext(ctx).Save(robot).Error
	}

	return ro, nil
}
