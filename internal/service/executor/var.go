package executor

import (
	"nightcord-server/utils"
	"sync"
)

var (
	folderLock sync.Mutex             // 文件夹创建锁
	random     = utils.LockedRandom{} // 线程安全的随机数生成器
)
