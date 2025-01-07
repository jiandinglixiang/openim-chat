package admin

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/openimsdk/chat/pkg/common/config"
	"github.com/openimsdk/chat/pkg/common/constant"
	"github.com/openimsdk/chat/pkg/common/db/database"
	"github.com/openimsdk/chat/pkg/common/db/dbutil"
	"github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/chat/pkg/common/tokenverify"
	adminpb "github.com/openimsdk/chat/pkg/protocol/admin"
	"github.com/openimsdk/chat/pkg/protocol/chat"
	chatClient "github.com/openimsdk/chat/pkg/rpclient/chat"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/db/redisutil"
	"github.com/openimsdk/tools/discovery"
	"github.com/openimsdk/tools/errs"
	"github.com/openimsdk/tools/mw"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	RpcConfig     config.Admin
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
	// 设置随机数种子
	rand.Seed(time.Now().UnixNano())
	// 初始化Redis客户端
	rdb, err := redisutil.NewRedisClient(ctx, config.RedisConfig.Build())
	if err != nil {
		return err // 如果初始化失败，返回错误
	}
	// 初始化MongoDB客户端
	mgocli, err := mongoutil.NewMongoDB(ctx, config.MongodbConfig.Build())
	if err != nil {
		return err // 如果初始化失败，返回错误
	}
	var srv adminServer
	// 配置Token验证信息
	srv.Token = &tokenverify.Token{
		Expires: time.Duration(config.RpcConfig.TokenPolicy.Expire) * time.Hour * 24, // Token过期时间
		Secret:  config.RpcConfig.Secret,                                             // Token密钥
	}
	// 初始化管理员数据库
	srv.Database, err = database.NewAdminDatabase(mgocli, rdb, srv.Token)
	if err != nil {
		return err // 如果初始化失败，返回错误
	}
	// 获取聊天服务的连接
	conn, err := client.GetConn(ctx, config.Share.RpcRegisterName.Chat, grpc.WithTransportCredentials(insecure.NewCredentials()), mw.GrpcClient())
	if err != nil {
		return err // 如果获取连接失败，返回错误
	}
	// 创建聊天客户端
	srv.Chat = chatClient.NewChatClient(chat.NewChatClient(conn))
	// 初始化管理员账户
	if err := srv.initAdmin(ctx, config.Share.ChatAdmin, config.Share.OpenIM.AdminUserID); err != nil {
		return err // 如果初始化失败，返回错误
	}
	// 注册管理员服务
	adminpb.RegisterAdminServer(server, &srv)
	return nil // 返回nil表示启动成功
}

type adminServer struct {
	Database database.AdminDatabaseInterface
	Chat     *chatClient.ChatClient
	Token    *tokenverify.Token
}

func (o *adminServer) initAdmin(ctx context.Context, admins []string, imUserID string) error {
	for _, account := range admins {
		if _, err := o.Database.GetAdmin(ctx, account); err == nil {
			continue
		} else if !dbutil.IsDBNotFound(err) {
			return err
		}
		sum := md5.Sum([]byte(account))
		a := admin.Admin{
			Account:    account,
			UserID:     imUserID,
			Password:   hex.EncodeToString(sum[:]),
			Level:      constant.DefaultAdminLevel,
			CreateTime: time.Now(),
		}
		if err := o.Database.AddAdminAccount(ctx, []*admin.Admin{&a}); err != nil {
			return err
		}
	}
	return nil
}
