package mr

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Coordinator struct {
	// Your definitions here.
	NReduce      int              //表示Reduce任务的个数
	MNum         map[string]int32 //表示最新map的任务编号
	RNum         int32            //表示最新reduce任务编号的分配
	FilesMap     SafeMap          //用来确认文件是否被分配,
	ReduceMap    map[int32]bool   //用来确认Reduce任务是否完成
	RDoneCount   int32            //表示reduce任务完成的数量
	MDoneCount   int32            //表示Mapper任务完成的数量
	*MapQueue                     //表示处理文件的队列
	*ReduceQueue                  //表示重启任务队列
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

func (c *Coordinator) AssignedTasks(args *AssignedArgs, reply *AssignedReply) error {

	//为当前节点分配任务
	s := c.AssignJobs()

	// fmt.Printf("num is %v \n", s)
	if s == "m" {
		//分配mapper任务
		var file string
		for true {
			file, _ = c.MapQueue.Dequeue()
			if file == "" {
				//again
				fmt.Println("文件队列为空")
				reply.Task = "a"
				return nil
			}
			err := c.FilesMap.SetTrue(file)
			if err != nil {
				//任务已经被分配,重新分配
				continue
			}
			break
		}
		//分配成功后，启动任务监听器，监听是否正常运行
		num := c.AssignTaskNum(s, file)
		reply.TaskNun = num
		reply.Task = "m"
		reply.File = file
		reply.NReduce = c.NReduce
		go c.MapTaskListner(file)
		return nil

	} else if s == "r" {
		//分配reduce任务

		num := c.AssignTaskNum("r", "")
		if num == -1 {
			//队列为空，重新分配
			reply.Task = "a"
			return nil
		}
		reply.File = fmt.Sprintf("mr-*-%v", num)
		reply.Task = "r"
		reply.TaskNun = num
		reply.NReduce = c.NReduce
		go c.ReduceTaskListner(num)
		return nil
	}
	reply.Task = "e"
	return nil
}

func (c *Coordinator) MapperDone(args *MapperDoneArgs, reply *MapperDoneReply) error {

	err := c.FilesMap.DeleteKey(args.File)
	fmt.Printf("%v is delete, \n", args.File)

	if err != nil {
		reply.IsDone = false
	}
	reply.IsDone = true
	atomic.AddInt32(&c.MDoneCount, 1)
	return nil
}

func (c *Coordinator) ReduceDone(args *ReduceDoneArgs, reply *ReduceDoneReply) error {

	c.ReduceMap[args.TaskNum] = true
	reply.IsDone = true
	atomic.AddInt32(&c.RDoneCount, 1)
	return nil
}

func (c *Coordinator) WorkerDone(args *ReduceDoneArgs, reply *ReduceDoneReply) error {

	if c.MapQueue.count <= 0 && c.RNum >= int32(c.NReduce) &&
		c.ReduceQueue.count <= 0 && c.RDoneCount == int32(c.NReduce) {

		reply.IsDone = true
		return nil
	}
	reply.IsDone = false
	return nil

}

func (c *Coordinator) MapTaskListner(task string) {

	time.Sleep(10 * time.Second)
	//检测mapper有没有在10s完成任务

	_, err := c.FilesMap.Get(task)
	if err == nil {
		//10s内没有完成任务，需要重新分配任务
		c.FilesMap.SetFalse(task)
		fmt.Printf("Map %v号没有完成作业，加入队列 %v\n", task, err)
		c.MapQueue.Enqueue(task)
	}
}

func (c *Coordinator) ReduceTaskListner(taskNum int32) {

	time.Sleep(10 * time.Second)
	//检测reduce有没有在10s完成任务

	//没有完成的，进入reduce任务重启队列
	if !c.ReduceMap[taskNum] {
		fmt.Printf("Reduce %v号没有完成作业，加入队列\n", taskNum)
		c.ReduceQueue.Enqueue(taskNum)
		fmt.Printf("queue 大小为%v", c.ReduceQueue.count)
	}
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	if c.MapQueue.count <= 0 && c.RNum >= int32(c.NReduce) &&
		c.ReduceQueue.count <= 0 && c.RDoneCount == int32(c.NReduce) {
		ret = true
		//给充足的时间让worker成功下线
		time.Sleep(time.Second * 6)
		fmt.Println("coordinator 即将下线")
		return ret
	}
	// Your code here.
	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {

	c := Coordinator{}
	fileMap := make(map[string]bool, len(files))
	queue := NewMapQueue(len(files))
	rqueue := NewReduceQueue(nReduce)
	reduceMap := make(map[int32]bool, nReduce)
	mNum := make(map[string]int32, len(files))
	// Your code here.

	//将待处理文件列表写入任务队列
	for index, file := range files {
		fileMap[file] = false
		queue.Enqueue(file)
		mNum[file] = int32(index) + 1
	}
	for i := 1; i <= nReduce; i++ {
		reduceMap[int32(i)] = false
	}

	c.NReduce = nReduce
	c.FilesMap.m = fileMap
	c.MapQueue = queue
	c.ReduceQueue = rqueue
	c.ReduceMap = reduceMap
	c.MNum = mNum

	c.server()
	return &c
}

func (c *Coordinator) AssignJobs() string {
	// fmt.Printf("%v --- %v --- %v --", c.RNum, c.MapQueue.count, c.ReduceQueue.count)
	if c.RNum >= int32(c.NReduce) && c.MapQueue.count <= 0 &&
		c.ReduceQueue.count <= 0 && c.RDoneCount >= int32(c.NReduce) {
		//exit
		return "e"
	}
	if c.MapQueue.count > 0 {
		//优先处理Mapper 任务
		return "m"
	}
	if c.MDoneCount < int32(c.MapQueue.size) {
		//表示中途有map奔溃，需要重新分配
		time.Sleep(time.Second * 8)
		fmt.Printf("MdoneCount %v\n", c.MDoneCount)
		if c.MDoneCount < int32(c.MapQueue.size) {
			keys, _ := c.FilesMap.GetAllKeys()
			//加入队列
			for _, v := range keys {
				j, _ := c.FilesMap.Get(v)
				if j != true {
					c.MapQueue.Enqueue(v)
				}
			}
			return "m"
		}

		return "r"

	}
	return "r"
}

func (c *Coordinator) AssignTaskNum(task string, file string) int32 {
	if task == "m" {
		return c.MNum[file]
	}

	if c.RNum >= int32(c.NReduce) {
		//从重启任务队列分配编号
		i, err := c.ReduceQueue.Dequeue()
		if err != nil {
			return -1
		}
		fmt.Printf("%v号被重新分配 \n", i)
		return i
	}
	i := atomic.AddInt32(&c.RNum, 1)
	return i
}

//-----------------------------------------------------------
type MapQueue struct {
	items     []string
	size      int
	headIndex int
	tailIndex int
	count     int
}

func NewMapQueue(size int) *MapQueue {
	return &MapQueue{
		items:     make([]string, size),
		size:      size,
		headIndex: 0,
		tailIndex: 0,
		count:     0,
	}
}

func (mq *MapQueue) Enqueue(item string) bool {
	if mq.count == mq.size {
		return false // 队列已满
	}

	mq.items[mq.tailIndex] = item
	mq.tailIndex = (mq.tailIndex + 1) % mq.size
	mq.count++
	return true
}

func (mq *MapQueue) Dequeue() (string, bool) {
	if mq.count == 0 {
		return "", false // 队列为空
	}

	item := mq.items[mq.headIndex]
	mq.headIndex = (mq.headIndex + 1) % mq.size
	mq.count--
	return item, true
}

type SafeMap struct {
	mu sync.RWMutex
	m  map[string]bool
}

func (sm *SafeMap) Get(key string) (bool, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	value, ok := sm.m[key]
	if ok == false {
		return value, errors.New("未找到key: " + key)
	}
	return value, nil
}

func (sm *SafeMap) GetAllKeys() ([]string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keys := make([]string, 0, len(sm.m))
	for k := range sm.m {
		keys = append(keys, k)
	}
	return keys, nil
}

func (sm *SafeMap) DeleteKey(key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.m[key]; ok {
		// key 存在于 myMap 中
		delete(sm.m, key)
	} else {
		// key 不存在于 myMap 中
		fmt.Printf("key is no exist")
		return errors.New("key is no exist")
	}
	return nil
}

func (sm *SafeMap) SetTrue(key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.m[key] == true {
		return errors.New("任务已经被分配")
	}
	sm.m[key] = true
	return nil
}

func (sm *SafeMap) SetFalse(key string) error {
	_, ok := sm.m[key]
	if ok == false {
		return errors.New("key 已经不存在")
	}
	sm.m[key] = false
	return nil

}

type ReduceQueue struct {
	queue []int32
	front int
	rear  int
	count int
	size  int
	lock  sync.Mutex
}

func NewReduceQueue(size int) *ReduceQueue {
	return &ReduceQueue{
		queue: make([]int32, size),
		front: 0,
		rear:  0,
		count: 0,
		size:  size,
		lock:  sync.Mutex{},
	}
}

func (q *ReduceQueue) Enqueue(item int32) error {

	if q.count == q.size {
		return errors.New("Queue is full")
	}
	q.queue[q.rear] = item
	q.rear = (q.rear + 1) % len(q.queue)
	q.count++
	return nil
}

func (q *ReduceQueue) Dequeue() (int32, error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.count == 0 {
		return -1, errors.New("Queue is empty")
	}
	item := q.queue[q.front]
	q.front = (q.front + 1) % len(q.queue)
	q.count--

	return item, nil
}
