package admin

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
	// 设置管理员路由
	SetAdminRoute(engine, adminApi, mwApi)
	// 启动Gin引擎并监听指定端口
	return engine.Run(fmt.Sprintf(":%d", apiPort))
}

func SetAdminRoute(router gin.IRouter, admin *Api, mw *chatmw.MW) {

	// 管理员账户相关路由组
	adminRouterGroup := router.Group("/account")
	adminRouterGroup.POST("/login", admin.AdminLogin)                                   // 管理员登录
	adminRouterGroup.POST("/update", mw.CheckAdmin, admin.AdminUpdateInfo)              // 更新管理员信息
	adminRouterGroup.POST("/info", mw.CheckAdmin, admin.AdminInfo)                      // 获取管理员信息
	adminRouterGroup.POST("/change_password", mw.CheckAdmin, admin.ChangeAdminPassword) // 更改管理员密码
	adminRouterGroup.POST("/add_admin", mw.CheckAdmin, admin.AddAdminAccount)           // 添加新的管理员账户
	adminRouterGroup.POST("/add_user", mw.CheckAdmin, admin.AddUserAccount)             // 添加新的用户账户
	adminRouterGroup.POST("/del_admin", mw.CheckAdmin, admin.DelAdminAccount)           // 删除管理员账户
	adminRouterGroup.POST("/search", mw.CheckAdmin, admin.SearchAdminAccount)           // 搜索管理员账户列表

	// 用户导入相关路由组
	importGroup := router.Group("/user/import")
	importGroup.POST("/json", mw.CheckAdmin, admin.ImportUserByJson) // 通过JSON导入用户
	importGroup.POST("/xlsx", mw.CheckAdmin, admin.ImportUserByXlsx) // 通过XLSX导入用户
	importGroup.GET("/xlsx", admin.BatchImportTemplate)              // 获取批量导入模板

	// 用户注册许可相关路由组
	allowRegisterGroup := router.Group("/user/allow_register", mw.CheckAdmin)
	allowRegisterGroup.POST("/get", admin.GetAllowRegister) // 获取允许注册的设置
	allowRegisterGroup.POST("/set", admin.SetAllowRegister) // 设置允许注册的配置

	// 默认好友和群组相关路由组
	defaultRouter := router.Group("/default", mw.CheckAdmin)
	defaultUserRouter := defaultRouter.Group("/user")
	defaultUserRouter.POST("/add", admin.AddDefaultFriend)       // 添加默认好友
	defaultUserRouter.POST("/del", admin.DelDefaultFriend)       // 删除默认好友
	defaultUserRouter.POST("/find", admin.FindDefaultFriend)     // 查找默认好友
	defaultUserRouter.POST("/search", admin.SearchDefaultFriend) // 搜索默认好友
	defaultGroupRouter := defaultRouter.Group("/group")
	defaultGroupRouter.POST("/add", admin.AddDefaultGroup)       // 添加默认群组
	defaultGroupRouter.POST("/del", admin.DelDefaultGroup)       // 删除默认群组
	defaultGroupRouter.POST("/find", admin.FindDefaultGroup)     // 查找默认群组
	defaultGroupRouter.POST("/search", admin.SearchDefaultGroup) // 搜索默认群组

	// 邀请码管理相关路由组
	invitationCodeRouter := router.Group("/invitation_code", mw.CheckAdmin)
	invitationCodeRouter.POST("/add", admin.AddInvitationCode)       // 添加邀请码
	invitationCodeRouter.POST("/gen", admin.GenInvitationCode)       // 生成邀请码
	invitationCodeRouter.POST("/del", admin.DelInvitationCode)       // 删除邀请码
	invitationCodeRouter.POST("/search", admin.SearchInvitationCode) // 搜索邀请码

	// 禁止IP和用户登录相关路由组
	forbiddenRouter := router.Group("/forbidden", mw.CheckAdmin)
	ipForbiddenRouter := forbiddenRouter.Group("/ip")
	ipForbiddenRouter.POST("/add", admin.AddIPForbidden)       // 添加禁止的IP
	ipForbiddenRouter.POST("/del", admin.DelIPForbidden)       // 删除禁止的IP
	ipForbiddenRouter.POST("/search", admin.SearchIPForbidden) // 搜索禁止的IP
	userForbiddenRouter := forbiddenRouter.Group("/user")
	userForbiddenRouter.POST("/add", admin.AddUserIPLimitLogin)       // 添加用户IP登录限制
	userForbiddenRouter.POST("/del", admin.DelUserIPLimitLogin)       // 删除用户IP登录限制
	userForbiddenRouter.POST("/search", admin.SearchUserIPLimitLogin) // 搜索用户IP登录限制

	// 小程序管理相关路由组
	appletRouterGroup := router.Group("/applet", mw.CheckAdmin)
	appletRouterGroup.POST("/add", admin.AddApplet)       // 添加小程序
	appletRouterGroup.POST("/del", admin.DelApplet)       // 删除小程序
	appletRouterGroup.POST("/update", admin.UpdateApplet) // 更新小程序
	appletRouterGroup.POST("/search", admin.SearchApplet) // 搜索小程序

	// 黑名单 相关路由组
	blockRouter := router.Group("/block", mw.CheckAdmin)
	blockRouter.POST("/add", admin.BlockUser)          // 阻止用户
	blockRouter.POST("/del", admin.UnblockUser)        // 解锁用户
	blockRouter.POST("/search", admin.SearchBlockUser) // 搜索被阻止的用户

	// 用户密码管理相关路由组
	userRouter := router.Group("/user", mw.CheckAdmin)
	userRouter.POST("/password/reset", admin.ResetUserPassword) // 重置用户密码

	// 客户端配置管理相关路由组
	initGroup := router.Group("/client_config", mw.CheckAdmin)
	initGroup.POST("/get", admin.GetClientConfig) // 获取客户端配置
	initGroup.POST("/set", admin.SetClientConfig) // 设置客户端配置
	initGroup.POST("/del", admin.DelClientConfig) // 删除客户端配置

	// 统计信息相关路由组
	statistic := router.Group("/statistic", mw.CheckAdmin)
	statistic.POST("/new_user_count", admin.NewUserCount)     // 新用户统计
	statistic.POST("/login_user_count", admin.LoginUserCount) // 登录用户统计

	// 应用版本管理相关路由组
	applicationGroup := router.Group("application")
	applicationGroup.POST("/add_version", mw.CheckAdmin, admin.AddApplicationVersion)       // 添加应用版本
	applicationGroup.POST("/update_version", mw.CheckAdmin, admin.UpdateApplicationVersion) // 更新应用版本
	applicationGroup.POST("/delete_version", mw.CheckAdmin, admin.DeleteApplicationVersion) // 删除应用版本
	applicationGroup.POST("/latest_version", admin.LatestApplicationVersion)                // 获取最新应用版本
	applicationGroup.POST("/page_versions", admin.PageApplicationVersion)                   // 分页获取应用版本
}
