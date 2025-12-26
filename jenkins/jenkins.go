package jenkins

import (
	"context"
	httpc "devops/tools/httpclient"
	"devops/tools/ioc"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bndr/gojenkins"

	c "devops/feishu/config"
)

type AppName string

const (
	AppNameJenkins AppName = "jenkins_client"
)

func init() {
	// Initialize gojenkins loggers to prevent panic
	gojenkins.Error = log.New(os.Stderr, "JENKINS ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	gojenkins.Warning = log.New(os.Stdout, "JENKINS WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	gojenkins.Info = log.New(os.Stdout, "JENKINS INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ioc.Api.RegisterContainer(string(AppNameJenkins), NewClient())

}

type Client struct {
	jenkins *gojenkins.Jenkins
}

func (c *Client) Init() error {
	return nil
}

// Job监控器
type JobSupervisor struct {
	Client *Client
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// 创建 JobSupervisor
func NewJobSupervisor(client *Client) *JobSupervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobSupervisor{
		Client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

// NewClient 创建 Jenkins 客户端
func NewClient() *Client {
	httpClient := httpc.CreateClient()

	// 创建 http.Client 的副本以修改 CheckRedirect，而不影响全局 client
	// 这样可以防止自动跟随重定向，从而捕获 Location 头
	clientCopy := *httpClient
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	cfg, err := c.LoadConfig()
	if err != nil {
		log.Printf("无法加载配置: %v", err)
		return nil
	}

	jenkins := gojenkins.CreateJenkins(&clientCopy, cfg.JenkinsURL, cfg.JenkinsUser, cfg.JenkinsToken)
	return &Client{
		jenkins: jenkins,
	}
}

// BuildRequest 结构体定义
type BuildRequest struct {
	JobName      string `json:"job_name"`
	Branch       string `json:"branch"`
	DeployType   string `json:"deploy_type"`
	ImageVersion string `json:"image_version"`
}

// BuildHandler 函数
func (c *Client) Build(ctx context.Context, req BuildRequest) (int64, error) {
	jenkins := c.jenkins

	ctx, cancel := context.WithTimeout(ctx, 30*1e9)
	defer cancel()

	// 获取指定 Job 的信息
	job, err := jenkins.GetJob(ctx, req.JobName)
	if err != nil {
		log.Printf("无法获取 Job '%s': %v", req.JobName, err)
		return 0, err
	}

	// 为指定 Job 和分支构建
	params := map[string]string{
		"BRANCH":        req.Branch,
		"DEPLOY_TYPE":   req.DeployType,
		"IMAGE_VERSION": req.ImageVersion,
	}

	var invokeErr error
	var queueID int64
	backoff := []int{1, 2, 4}

	// 构造 JSON 格式参数
	type NameValue struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type ParameterHolder struct {
		Parameter []NameValue `json:"parameter"`
	}
	holder := ParameterHolder{}
	for k, v := range params {
		holder.Parameter = append(holder.Parameter, NameValue{Name: k, Value: v})
	}
	jsonBytes, _ := json.Marshal(holder)

	// 准备 POST 数据
	data := url.Values{}
	data.Set("json", string(jsonBytes))

	for i := 0; i < len(backoff); i++ {
		// 使用底层 Requester 直接触发构建
		// 注意：使用 /build 接口配合 json 参数，可以绕过某些插件（如 Git Parameter）的严格校验
		// 并且能够正确传递参数
		endpoint := job.Base + "/build"
		payload := strings.NewReader(data.Encode())
		resp, err := jenkins.Requester.Post(ctx, endpoint, payload, nil, nil)

		if err == nil {
			// 201 Created is standard, but some Jenkins configs return 302 Found (redirect to queue)
			// We handle 302 because we disabled redirect following
			if (resp.StatusCode >= 200 && resp.StatusCode < 300) || resp.StatusCode == 302 {
				// 提取 Queue Item ID
				location := resp.Header.Get("Location")
				if location == "" {
					location = resp.Header.Get("location")
				}

				if location == "" {
					// 如果没有 Location 头，且状态码是 200，可能是因为 http client 自动跟随了重定向
					// 此时检查最终请求的 URL
					if resp.StatusCode == 200 && resp.Request != nil && resp.Request.URL != nil {
						location = resp.Request.URL.String()
					}
				}

				if location == "" {
					invokeErr = fmt.Errorf("jenkins did not return Location header (status: %d)", resp.StatusCode)
				} else {
					// Location should be like: http://jenkins/queue/item/123/
					// 如果 Location 指向 Job 本身 (e.g. /job/xxx/)，说明 Jenkins 没有创建新队列项，
					// 可能是因为静默期、或者认为参数无变化无需构建（取决于插件），或者构建已经瞬间完成？
					// 但用户反馈是构建确实触发了。
					// 这种情况下，我们需要一种 fallback 机制来找到这个队列项。

					if !strings.Contains(location, "/queue/item/") {
						log.Printf("Warning: Location header '%s' does not contain '/queue/item/'. Trying to find queue item by scanning queue...", location)

						// 尝试从 Job 的队列中查找
						// 先等待一小会儿，确保 Jenkins 更新了队列
						time.Sleep(2 * time.Second)

						queue, err := jenkins.GetQueue(ctx)
						if err == nil {
							// 遍历队列寻找匹配 JobName 的任务
							// 注意：这里可能有多条，我们取最新的一条（ID 最大的）
							var foundID int64
							for _, item := range queue.Raw.Items {
								if item.Task.Name == req.JobName {
									if item.ID > foundID {
										foundID = item.ID
									}
								}
							}
							if foundID > 0 {
								queueID = foundID
								invokeErr = nil
								break
							}
						}

						// 如果还是没找到，可能构建已经开始了？
						// 尝试查找最新的构建
						lastBuild, err := job.GetLastBuild(ctx)
						if err == nil && lastBuild != nil {
							// 如果最新的构建正在运行或者刚刚开始（假设时间差很短），我们可以认为就是它
							// 但没有 QueueID 我们无法调用 WaitForBuildToStart(queueID)
							// 这是一个棘手的情况。
							// 这里我们暂时只能报错，或者伪造一个 QueueID (不推荐)
							invokeErr = fmt.Errorf("failed to parse queue id from location '%s' and could not find in queue", location)
						} else {
							invokeErr = fmt.Errorf("failed to parse queue id from location '%s'", location)
						}

					} else {
						// 正常解析 Queue ID
						parts := strings.Split(strings.TrimRight(location, "/"), "/")
						if len(parts) > 0 {
							idStr := parts[len(parts)-1]
							queueID, err = strconv.ParseInt(idStr, 10, 64)
							if err != nil {
								invokeErr = fmt.Errorf("failed to parse queue id from location '%s': %v", location, err)
							} else {
								invokeErr = nil
								break
							}
						} else {
							invokeErr = fmt.Errorf("invalid Location header format: %s", location)
						}
					}
				}
			} else {
				invokeErr = fmt.Errorf("jenkins returned status code: %d", resp.StatusCode)
			}
		} else {
			invokeErr = err
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		time.Sleep(time.Duration(backoff[i]) * time.Second)
	}
	if invokeErr != nil {
		log.Printf("无法启动 Job '%s' 的构建: %v", req.JobName, invokeErr)
		return 0, invokeErr
	}

	log.Printf("Build triggered for job '%s' with branch '%s' (DeployType: %s). Queue ID: %d", req.JobName, req.Branch, req.DeployType, queueID)
	return queueID, nil
}

// WaitForBuildToStart 等待构建开始并返回构建号
func (c *Client) WaitForBuildToStart(ctx context.Context, queueID int64) (int64, error) {
	jenkins := c.jenkins
	backoff := 1 * time.Second
	maxRetries := 180 // 等待 180 秒

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		task, err := jenkins.GetQueueItem(ctx, queueID)
		if err != nil {
			// 可能是临时的网络错误，重试
			log.Printf("Failed to get queue item %d: %v", queueID, err)
			time.Sleep(backoff)
			continue
		}

		if task.Raw.Executable.Number != 0 {
			return task.Raw.Executable.Number, nil
		}

		// 某些原因可能导致任务不在队列中但已经开始？通常 gojenkins 会处理这个。
		// 如果 Why 字段不为空，说明还在等待
		// log.Printf("Waiting for build to start... Why: %s", task.Raw.Why)

		time.Sleep(backoff)
	}

	return 0, fmt.Errorf("timeout waiting for build to start (queue id: %d)", queueID)
}

// GetJobBuildInfo 获取 Job 的构建信息
// jobName: Job 名称
// buildNumber: 构建号
// 返回值: 构建信息结构体指针和错误信息
func (c *Client) GetJobBuildInfo(ctx context.Context, jobName string, buildNumber int) (*gojenkins.Build, error) {
	job, err := c.jenkins.GetJob(ctx, jobName)
	if err != nil {
		return nil, err
	}

	build, err := job.GetBuild(ctx, int64(buildNumber))
	if err != nil {
		return nil, err
	}

	return build, nil
}
