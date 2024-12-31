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
	if len(config.Share.ChatAdmin) == 0 {
		return errs.New("share chat admin not configured")
	}
	apiPort, err := datautil.GetElemByIndex(config.ApiConfig.Api.Ports, index)
	if err != nil {
		return err
	}
	client, err := kdisc.NewDiscoveryRegister(&config.Discovery)
	if err != nil {
		return err
	}

	chatConn, err := client.GetConn(ctx, config.Share.RpcRegisterName.Chat, grpc.WithTransportCredentials(insecure.NewCredentials()), mw.GrpcClient())
	if err != nil {
		return err
	}
	adminConn, err := client.GetConn(ctx, config.Share.RpcRegisterName.Admin, grpc.WithTransportCredentials(insecure.NewCredentials()), mw.GrpcClient())
	if err != nil {
		return err
	}
	chatClient := chatclient.NewChatClient(chatConn)
	adminClient := adminclient.NewAdminClient(adminConn)
	im := imapi.New(config.Share.OpenIM.ApiURL, config.Share.OpenIM.Secret, config.Share.OpenIM.AdminUserID)
	base := util.Api{
		ImUserID:        config.Share.OpenIM.AdminUserID,
		ProxyHeader:     config.Share.ProxyHeader,
		ChatAdminUserID: config.Share.ChatAdmin[0],
	}
	adminApi := New(chatClient, adminClient, im, &base)
	mwApi := chatmw.New(adminClient)
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery(), mw.CorsHandler(), mw.GinParseOperationID())
	SetAdminRoute(engine, adminApi, mwApi)
	return engine.Run(fmt.Sprintf(":%d", apiPort))
}

func SetAdminRoute(router gin.IRouter, admin *Api, mw *chatmw.MW) {

	adminRouterGroup := router.Group("/account")
	adminRouterGroup.POST("/login", admin.AdminLogin)                                   // 登录
	adminRouterGroup.POST("/update", mw.CheckAdmin, admin.AdminUpdateInfo)              // 修改信息
	adminRouterGroup.POST("/info", mw.CheckAdmin, admin.AdminInfo)                      // 获取信息
	adminRouterGroup.POST("/change_password", mw.CheckAdmin, admin.ChangeAdminPassword) // 更改管理员帐户的密码
	adminRouterGroup.POST("/add_admin", mw.CheckAdmin, admin.AddAdminAccount)           // 添加管理员帐户
	adminRouterGroup.POST("/add_user", mw.CheckAdmin, admin.AddUserAccount)             // 添加用户帐户
	adminRouterGroup.POST("/del_admin", mw.CheckAdmin, admin.DelAdminAccount)           // 删除管理员
	adminRouterGroup.POST("/search", mw.CheckAdmin, admin.SearchAdminAccount)           // 获取管理员列表
	//account.POST("/add_notification_account")

	importGroup := router.Group("/user/import")
	importGroup.POST("/json", mw.CheckAdmin, admin.ImportUserByJson)
	importGroup.POST("/xlsx", mw.CheckAdmin, admin.ImportUserByXlsx)
	importGroup.GET("/xlsx", admin.BatchImportTemplate)

	allowRegisterGroup := router.Group("/user/allow_register", mw.CheckAdmin)
	allowRegisterGroup.POST("/get", admin.GetAllowRegister)
	allowRegisterGroup.POST("/set", admin.SetAllowRegister)

	defaultRouter := router.Group("/default", mw.CheckAdmin)
	defaultUserRouter := defaultRouter.Group("/user")
	defaultUserRouter.POST("/add", admin.AddDefaultFriend)       // 注册时添加默认好友
	defaultUserRouter.POST("/del", admin.DelDefaultFriend)       // 删除注册时的默认好友
	defaultUserRouter.POST("/find", admin.FindDefaultFriend)     // 默认好友列表
	defaultUserRouter.POST("/search", admin.SearchDefaultFriend) // 注册时搜索默认好友列表
	defaultGroupRouter := defaultRouter.Group("/group")
	defaultGroupRouter.POST("/add", admin.AddDefaultGroup)       // 注册时添加默认组
	defaultGroupRouter.POST("/del", admin.DelDefaultGroup)       // 删除注册时的默认组
	defaultGroupRouter.POST("/find", admin.FindDefaultGroup)     // 注册时获取默认群组列表
	defaultGroupRouter.POST("/search", admin.SearchDefaultGroup) // 注册时搜索默认群组列表

	invitationCodeRouter := router.Group("/invitation_code", mw.CheckAdmin)
	invitationCodeRouter.POST("/add", admin.AddInvitationCode)       // 添加邀请码
	invitationCodeRouter.POST("/gen", admin.GenInvitationCode)       // 生成邀请码
	invitationCodeRouter.POST("/del", admin.DelInvitationCode)       // 删除邀请码
	invitationCodeRouter.POST("/search", admin.SearchInvitationCode) // 搜索邀请码

	forbiddenRouter := router.Group("/forbidden", mw.CheckAdmin)
	ipForbiddenRouter := forbiddenRouter.Group("/ip")
	ipForbiddenRouter.POST("/add", admin.AddIPForbidden)       // 添加禁止注册/登录IP
	ipForbiddenRouter.POST("/del", admin.DelIPForbidden)       // 删除禁止注册/登录的IP
	ipForbiddenRouter.POST("/search", admin.SearchIPForbidden) // 搜索禁止的IP进行注册/登录
	userForbiddenRouter := forbiddenRouter.Group("/user")
	userForbiddenRouter.POST("/add", admin.AddUserIPLimitLogin)       // 添加限制特定IP的用户登录
	userForbiddenRouter.POST("/del", admin.DelUserIPLimitLogin)       // 删除特定IP登录的用户限制
	userForbiddenRouter.POST("/search", admin.SearchUserIPLimitLogin) // 特定IP的用户登录搜索限制

	appletRouterGroup := router.Group("/applet", mw.CheckAdmin)
	appletRouterGroup.POST("/add", admin.AddApplet)       // 添加小程序
	appletRouterGroup.POST("/del", admin.DelApplet)       // 删除小程序
	appletRouterGroup.POST("/update", admin.UpdateApplet) // 修改小程序
	appletRouterGroup.POST("/search", admin.SearchApplet) // 搜索小程序

	blockRouter := router.Group("/block", mw.CheckAdmin)
	blockRouter.POST("/add", admin.BlockUser)          // 阻止用户
	blockRouter.POST("/del", admin.UnblockUser)        // 解锁用户
	blockRouter.POST("/search", admin.SearchBlockUser) // 搜索被阻止的用户

	userRouter := router.Group("/user", mw.CheckAdmin)
	userRouter.POST("/password/reset", admin.ResetUserPassword) // 重置用户密码

	initGroup := router.Group("/client_config", mw.CheckAdmin)
	initGroup.POST("/get", admin.GetClientConfig) // 获取客户端初始化配置
	initGroup.POST("/set", admin.SetClientConfig) // 设置客户端初始化配置
	initGroup.POST("/del", admin.DelClientConfig) // 删除客户端初始化配置

	statistic := router.Group("/statistic", mw.CheckAdmin)
	statistic.POST("/new_user_count", admin.NewUserCount)
	statistic.POST("/login_user_count", admin.LoginUserCount)

	applicationGroup := router.Group("application")
	applicationGroup.POST("/add_version", mw.CheckAdmin, admin.AddApplicationVersion)
	applicationGroup.POST("/update_version", mw.CheckAdmin, admin.UpdateApplicationVersion)
	applicationGroup.POST("/delete_version", mw.CheckAdmin, admin.DeleteApplicationVersion)
	applicationGroup.POST("/latest_version", admin.LatestApplicationVersion)
	applicationGroup.POST("/page_versions", admin.PageApplicationVersion)
}
