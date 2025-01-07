package chat

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	chatmw "github.com/openimsdk/chat/internal/api/mw"
	"github.com/openimsdk/chat/internal/api/util"
	"github.com/openimsdk/chat/pkg/common/config"
	"github.com/openimsdk/chat/pkg/common/imapi"
	"github.com/openimsdk/chat/pkg/common/kdisc"
	adminclient "github.com/openimsdk/chat/pkg/protocol/admin"
	chatclient "github.com/openimsdk/chat/pkg/protocol/chat"
	"github.com/openimsdk/tools/errs"
	"github.com/openimsdk/tools/mw"
	"github.com/openimsdk/tools/utils/datautil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	ApiConfig config.API
	Discovery config.Discovery
	Share     config.Share
}

func Start(ctx context.Context, index int, config *Config) error {
	// 检查配置中是否有配置聊天管理员，如果没有则返回错误
	if len(config.Share.ChatAdmin) == 0 {
		return errs.New("share chat admin not configured") // 返回错误信息，表示未配置聊天管理员
	}
	// 获取API端口号
	apiPort, err := datautil.GetElemByIndex(config.ApiConfig.Api.Ports, index)
	if err != nil {
		return err // 如果获取端口号失败，返回错误
	}
	// 创建新的服务发现注册器
	client, err := kdisc.NewDiscoveryRegister(&config.Discovery)
	if err != nil {
		return err // 如果创建失败，返回错误
	}

	// 获取聊天服务的连接
	chatConn, err := client.GetConn(ctx, config.Share.RpcRegisterName.Chat, grpc.WithTransportCredentials(insecure.NewCredentials()), mw.GrpcClient())
	if err != nil {
		return err // 如果获取连接失败，返回错误
	}
	// 获取管理员服务的连接
	adminConn, err := client.GetConn(ctx, config.Share.RpcRegisterName.Admin, grpc.WithTransportCredentials(insecure.NewCredentials()), mw.GrpcClient())
	if err != nil {
		return err // 如果获取连接失败，返回错误
	}
	// 创建聊天客户端
	chatClient := chatclient.NewChatClient(chatConn)
	// 创建管理员客户端
	adminClient := adminclient.NewAdminClient(adminConn)
	// 创建IM API实例
	im := imapi.New(config.Share.OpenIM.ApiURL, config.Share.OpenIM.Secret, config.Share.OpenIM.AdminUserID)
	// 初始化基础API配置
	base := util.Api{
		ImUserID:        config.Share.OpenIM.AdminUserID, // IM用户ID
		ProxyHeader:     config.Share.ProxyHeader,        // 代理头
		ChatAdminUserID: config.Share.ChatAdmin[0],       // 聊天管理员用户ID
	}
	// 创建管理员API实例
	adminApi := New(chatClient, adminClient, im, &base)
	// 创建中间件API实例
	mwApi := chatmw.New(adminClient)
	// 设置Gin为发布模式
	gin.SetMode(gin.ReleaseMode)
	// 创建新的Gin引擎
	engine := gin.New()
	// 使用Gin的恢复中间件和自定义中间件
	engine.Use(gin.Recovery(), mw.CorsHandler(), mw.GinParseOperationID())
	// 设置聊天路由
	SetChatRoute(engine, adminApi, mwApi)
	// 启动Gin引擎并监听指定端口
	return engine.Run(fmt.Sprintf(":%d", apiPort))
}

func SetChatRoute(router gin.IRouter, chat *Api, mw *chatmw.MW) {
	account := router.Group("/account")
	account.POST("/code/send", chat.SendVerifyCode)                      // 发送验证码到用户
	account.POST("/code/verify", chat.VerifyCode)                        // 验证用户输入的验证码
	account.POST("/register", mw.CheckAdminOrNil, chat.RegisterUser)     // 用户注册
	account.POST("/login", chat.Login)                                   // 用户登录
	account.POST("/password/reset", chat.ResetPassword)                  // 重置用户密码
	account.POST("/password/change", mw.CheckToken, chat.ChangePassword) // 用户更改密码

	user := router.Group("/user", mw.CheckToken)
	user.POST("/update", chat.UpdateUserInfo)                 // 更新用户个人信息
	user.POST("/find/public", chat.FindUserPublicInfo)        // 获取用户的公开信息
	user.POST("/find/full", chat.FindUserFullInfo)            // 获取用户的完整信息
	user.POST("/search/full", chat.SearchUserFullInfo)        // 搜索用户的完整信息
	user.POST("/search/public", chat.SearchUserPublicInfo)    // 搜索用户的公开信息
	user.POST("/rtc/get_token", chat.GetTokenForVideoMeeting) // 获取视频会议的用户令牌

	router.POST("/friend/search", mw.CheckToken, chat.SearchFriend) // 搜索用户的朋友

	router.Group("/applet").POST("/find", mw.CheckToken, chat.FindApplet) // 获取小程序列表

	router.Group("/client_config").POST("/get", chat.GetClientConfig) // 获取客户端的初始化配置

	applicationGroup := router.Group("application")
	applicationGroup.POST("/latest_version", chat.LatestApplicationVersion) // 获取最新的应用版本信息
	applicationGroup.POST("/page_versions", chat.PageApplicationVersion)    // 分页获取应用版本信息

	router.Group("/callback").POST("/open_im", chat.OpenIMCallback) // 处理OpenIM的回调请求
}
