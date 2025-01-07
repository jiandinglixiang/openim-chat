package chat

import (
	"context"
	"time"

	"github.com/openimsdk/chat/pkg/common/mctx"
	"github.com/openimsdk/chat/pkg/common/rtc"
	"github.com/openimsdk/chat/pkg/protocol/admin"
	"github.com/openimsdk/chat/pkg/protocol/chat"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/discovery"
	"github.com/openimsdk/tools/errs"
	"github.com/openimsdk/tools/mw"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/openimsdk/chat/pkg/common/config"
	"github.com/openimsdk/chat/pkg/common/db/database"
	"github.com/openimsdk/chat/pkg/email"
	chatClient "github.com/openimsdk/chat/pkg/rpclient/chat"
	"github.com/openimsdk/chat/pkg/sms"
)

type Config struct {
	RpcConfig     config.Chat
	RedisConfig   config.Redis
	MongodbConfig config.Mongo
	Discovery     config.Discovery
	Share         config.Share
}

func Start(ctx context.Context, config *Config, client discovery.SvcDiscoveryRegistry, server *grpc.Server) error {
	// 检查配置中是否有配置聊天管理员，如果没有则返回错误
	if len(config.Share.ChatAdmin) == 0 {
		return errs.New("share chat admin not configured") // 返回错误信息，表示未配置聊天管理员
	}
	// 初始化MongoDB客户端
	mgocli, err := mongoutil.NewMongoDB(ctx, config.MongodbConfig.Build())
	if err != nil {
		return err // 如果初始化失败，返回错误
	}
	var srv chatSvr
	// 根据配置选择短信服务提供商
	switch config.RpcConfig.VerifyCode.Phone.Use {
	case "ali":
		ali := config.RpcConfig.VerifyCode.Phone.Ali
		// 初始化阿里云短信服务
		srv.SMS, err = sms.NewAli(ali.Endpoint, ali.AccessKeyID, ali.AccessKeySecret, ali.SignName, ali.VerificationCodeTemplateCode)
		if err != nil {
			return err // 如果初始化失败，返回错误
		}
	}
	// 如果邮件验证功能启用，初始化邮件服务
	if mail := config.RpcConfig.VerifyCode.Mail; mail.Enable {
		srv.Mail = email.NewMail(mail.SMTPAddr, mail.SMTPPort, mail.SenderMail, mail.SenderAuthorizationCode, mail.Title)
	}
	// 初始化聊天数据库
	srv.Database, err = database.NewChatDatabase(mgocli)
	if err != nil {
		return err // 如果初始化失败，返回错误
	}
	// 获取管理员服务的连接
	conn, err := client.GetConn(ctx, config.Share.RpcRegisterName.Admin, grpc.WithTransportCredentials(insecure.NewCredentials()), mw.GrpcClient())
	if err != nil {
		return err // 如果获取连接失败，返回错误
	}
	// 创建管理员客户端
	srv.Admin = chatClient.NewAdminClient(admin.NewAdminClient(conn))
	// 配置验证码相关参数
	srv.Code = verifyCode{
		UintTime:   time.Duration(config.RpcConfig.VerifyCode.UintTime) * time.Second,
		MaxCount:   config.RpcConfig.VerifyCode.MaxCount,
		ValidCount: config.RpcConfig.VerifyCode.ValidCount,
		SuperCode:  config.RpcConfig.VerifyCode.SuperCode,
		ValidTime:  time.Duration(config.RpcConfig.VerifyCode.ValidTime) * time.Second,
		Len:        config.RpcConfig.VerifyCode.Len,
	}
	// 初始化LiveKit服务
	srv.Livekit = rtc.NewLiveKit(config.RpcConfig.LiveKit.Key, config.RpcConfig.LiveKit.Secret, config.RpcConfig.LiveKit.URL)
	// 设置是否允许注册
	srv.AllowRegister = config.RpcConfig.AllowRegister
	// 注册聊天服务
	chat.RegisterChatServer(server, &srv)
	return nil // 返回nil表示启动成功
}

type chatSvr struct {
	Database        database.ChatDatabaseInterface
	Admin           *chatClient.AdminClient
	SMS             sms.SMS
	Mail            email.Mail
	Code            verifyCode
	Livekit         *rtc.LiveKit
	ChatAdminUserID string
	AllowRegister   bool
}

func (o *chatSvr) WithAdminUser(ctx context.Context) context.Context {
	return mctx.WithAdminUser(ctx, o.ChatAdminUserID)
}

type verifyCode struct {
	UintTime   time.Duration // sec
	MaxCount   int
	ValidCount int
	SuperCode  string
	ValidTime  time.Duration
	Len        int
}
