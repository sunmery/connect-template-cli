package service

import (
    "context"

    pb "api/helloworld/demo"
    democonnect "api/helloworld/demo/democonnect"

    "connectrpc.com/connect"
    "api/internal/biz"
)

// DemoService 实现 Connect 服务
type DemoService struct {
    // 业务逻辑依赖
    uc *biz.DemoUseCase
}

// 显式接口检查
var _ democonnect.DemoServiceHandler = (*DemoService)(nil)
