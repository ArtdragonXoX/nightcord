package executor

import (
	"nightcord-server/internal/model"
	"nightcord-server/utils"
	"sync"
)

var (
	jobQueue   chan *model.Job
	runQueue   chan *model.RunJob
	folderLock sync.Mutex             // 文件夹创建锁
	random     = utils.LockedRandom{} // 线程安全的随机数生成器
)
