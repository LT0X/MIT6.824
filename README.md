
#  MIT6.824

<br>

**全部Lab由本人全部独立完成，且没有借鉴互联网其他完成者的任何代码，全部代码编写和架构设计全部由本人独立完成**


<br>

## Lab1 MapReduce
### MapReduce 论文地址：
http://research.google.com/archive/mapreduce-osdi04.pdf
### 整体的架构设计

#### MapReduce的设计架构
![0242ce53052e02cbc3be329c0e7e8e1](https://github.com/LT0X/MIT6.824/assets/88709343/c99d1e53-5d4f-4f78-8a43-bba2ae4753b9)

#### MapReduce的技术细节

MapReduce分为Map和Reduce两个操作，首先进行Map操作，进行分类汇总生成临时Map处理的临时文件，然后在通过hash算法得到可以处理的Reduce，把要Map处理的文件发送给指定的Reduce，最终Reduce把处理的文件发送到GFS(全局文件系统)进行存储.


这次实现的Lab的主要实现一个MapReduce架构处理统计单词次数的系统，首先这个实现的MapReduce主要有两个部分主体，Coordinator和Worker,Coordinator是协调器，主要的用途是协调map任务和reduce任务的分配，以及对处理结果的统计,使协调两者的处理可以按预期的进行下去，Woker则是进行处理的主体，它会向Coordinator进行任务的申请，然后处理分配到的任务。

首先整个MapReduce 再开始启动的时候，Worker会通过RPC请求调用coordinator的`AssignedTasks()`函数进行请求任务的分配，这个时候coordinator会优先分配Map任务，因为只有Map任务完成以后，才可以进行Reduce任务，设计的时候，在coordinator会启动初始化的时候，它会创建两个任务队列，一个是ReduceQueue，这个之后再说,一个是MapQueue,这个是Map任务的任务队列，用来存储Map要完成的任务，它们的实现都是基于数组实现的一个循环队列，当初选择队列的原因是，因为要存储MapReduce的处理任务，同时里面的Map任务或者Reduce任务可能因为其他异常情况导致，导致任务的无法完成，这个时候就得把未完成的任务，重新加入到Coordinator,由于队列的先进先出的特点，很适合来来作为存储任务列表,


```js
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
```
`AssignedTasks()`会优先分配map任务，会从Map任务队列取出一个任务，同时会给本次的Task分配一个TaskNum,以便worker创建文件的时候发生命名冲突,然后对于每一个Map的请求，coordinator启动一个后台的协程来对此进行一个监控,如果在指定的时间内没有完成任务，coordinator会把这个任务重新放回任务队列里面，以便下一个worker可以从中获取任务进行相关的处理


```js

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

	}
        .............
}
```


```js
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

```
Worker接受到Map任务，就开始处理任务，首先它会根据分配的任务从文件系统中中找到对应的处理文件,由于本次的Lab并不像实际工程中分布式从GFS获取文件，只是在单机中进行模拟，所以就是从本机的文件系统获取处理文件，获取完文件就读取文件内容加载到内存中,对文件的内容进行词汇的提取，然后创建出NReduce个临时切片对象，然后对每一个提取出来的Value的key进行hash算法，其中key为提取出来的单词，hash会得到一个切片的索引，然后根据索引添加到之前创建出来的临时切片对象，全部处理完成之后，将这些NReduce个临时切片对象进行编码创建NReduce个Map处理文件到GFS中，等待指定Reduce进行处理，然后发送完成RPC请求到coordinator中进行统计数据。


```js
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for true {
		//分配任务
		reply := CallAssignedTasks()
		// fmt.Println(reply)
		if reply.Task == "m" {

			//分配到map任务
			kv := mapperHandler(reply.File, mapf)

			//创建Nreduce个reduce的键值对存储
			kvs := make([][]KeyValue, reply.NReduce)
			for i := 0; i < reply.NReduce; i++ {
				kvs[i] = []KeyValue{}
			}

			for _, v := range *kv {
				//根据hash 决定键值对由那个reduce统计
				rnum := ihash(v.Key) % reply.NReduce
				kvs[rnum] = append(kvs[rnum], v)
			}

			for i := 0; i < reply.NReduce; i++ {
				//创建Nreduce个临时文件
				fileName := fmt.Sprintf("mr-%v-%v", int(reply.TaskNun), i+1)
				createIntermediateFile(fileName, kvs[i])
			}

			res, _ := CallMapperDone(reply.File)
			if res.IsDone == true {
				continue
			}

		} else if reply.Task == "r" {

			.........

		} else if reply.Task == "a" {
			//重新申请任务
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	for res, _ := CallWorkerDone(); !res.IsDone; {
		//休眠两秒重新检测
		time.Sleep(time.Second * 2)
	}
	fmt.Println("worker 退出")
	return
}
```
如果处理完所有的Map任务，则要开始处理Reduce任务,进行Reduce任务的分配的时候，并不会直接从ReduceQueue 里面去取任务，ReduceQueue的创建是为了存储那些完成失败需要重新分配的Reduced任务，由于MapReduce会设置NReduce的数量，并且每一个map的任务的都会创建NReduce个Map文件，并且我的命名都是按顺序来命名的，所以我Reduce任务的分配只需要分配一个索引N编号就可以了,编号从0开始，每次分配原子自增1，如果编号超过了NReduce,则会从ReduceQueue里面检查是否有完成失败的任务需要重新分配。同时分配结果发送给Worker之前，会启动一个后台协程，会对reduce任务进行监控，如果指定的时间没有完成则会把任务入队到ReduceQueue，等待下次worker重新处理。


```js

func (c *Coordinator) AssignedTasks(args *AssignedArgs, reply *AssignedReply) error {

	//为当前节点分配任务
	s := c.AssignJobs()

	// fmt.Printf("num is %v \n", s)
	if s == "m" {
        
		............
                
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
```


```js

func (c *Coordinator) AssignTaskNum(task string, file string) int32 {
	
        ............

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
```

worker接受到Reduce任务，开始Reduce任务的处理流程，首先Reduce会读取所有自己处理的map处理的中间文件，把它加载到内存中,然后对这些数据进行Reduce操作，最后汇总成一个输出文件，然后将这个输出文件进行保存到GFS中。

```js
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for true {
		//分配任务
		reply := CallAssignedTasks()
		// fmt.Println(reply)
		if reply.Task == "m" {

			.........

		} else if reply.Task == "r" {

			//开始处理redue任务
			var allKv []KeyValue
			files, err := filepath.Glob(reply.File)
			if err != nil {
				fmt.Println("获取文件列表错误:", err)
				return
			}
			for _, file := range files {
				kv, err := readIntermediateFile(file)
				if err != nil {
					fmt.Printf("error : %v", err)
				}
				allKv = append(allKv, *kv...)
			}

			final := ReduceHandler(allKv, reducef)
			fileName := fmt.Sprintf("mr-out-%v", reply.TaskNun)
			createOutputFile(fileName, *final)
			res, _ := CallReduceDone(reply.TaskNun)
			if res.IsDone {
				continue
			}

		} else if reply.Task == "a" {
			//重新申请任务
			time.Sleep(time.Second * 2)
			continue

		}
		break
	}
	for res, _ := CallWorkerDone(); !res.IsDone; {
		//休眠两秒重新检测
		time.Sleep(time.Second * 2)
	}
	fmt.Println("worker 退出")
	return
}

```





## Lab2 Raft

### Raft 论文地址：
https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf
### 整体的架构设计


#### 选举架构设计图

![a27e364acaf307f751c1fb3cdb20a3f](https://github.com/LT0X/MIT6.824/assets/88709343/3c01059c-9230-4828-a6bb-8d5bacc5cdea)
   
#### raft的技术细节

 raft开始启动的时候，会看是不是是否有对raft的状态的持久化文件，有则进行持久化的文件的读,如果无则以F的身份启动。

要选举的时候，节点的任期会自增1（这个操作为原子操作），F会从F身份转变为“C”候选人的身份，然后会对其他的节点进行发送RPC请求
`rf.RequestVoteRunTime（）`

这里我的实现是通过循环遍历各个在raft里面其他的节点，对其他的除了自己的其他节点进行发送请求，同时为每一个RPC投票请求异步创建一个协程进行跟踪 

```js
			for k, _ := range rf.peers {
				//给所有节点发送选举请求
				if k == rf.me {
					continue
				}
				if rf.state != "C" {
					break
				}

				reply := RequestVoteReply{
					VoteGranted: false,
				}
				go rf.RequestVoteRunTime(k, &args, &reply)
                           }
```
 然后设置请求超时超时时间，然后进行投票后续的汇总，如果投票同意的结果大于等于总节点数n的1/2+1
则视为选举成功，选举成功后进行身份的转换，身份转换为leader，如何进行创建leader相关的后台协程

```js
                         ms1 := 100 + (rand.Int63() % 150)
			time.Sleep(time.Duration(ms1) * time.Millisecond)

			if rf.state == "C" && int(rf.voteNum) >= len(rf.peers)/2+1 {
				// fmt.Printf("index %v 成为 leader,voteNum : %v loglen :%v cueeterm %v\n ",
				// 	rf.me, rf.voteNum, rf.logLastIndex, rf.currentTerm)
				//选举通过,成为leader,重置属性
				rf.LeaderReset()
				rf.state = "L"
				rf.persist()
				go rf.sendHeartbeat()
				go rf.SendAppendLogsHandler()
				go rf.LCommitLogRunTime()
				go rf.InstallSnapshotRunTime()
				continue
			}
```

leader会在后台创建一个心跳协程，为每一个其他的F节点发送心跳请求，维持自己的leader位置，关于心跳的实现的我是通过raft维持一个两个变量，一个是上一次超时选举周期的接受心跳次数last，另外一个是现在的心跳now,每一次接受到leader心跳则进行now+1,经过一次超时选举周期，raft会对调用`isNormal()`
进行判断，如果h.now > h.last 则表示心跳正常，相反则表示选举超时，需要进行term自增，进行选举，

同时如果L收到节点心跳返回的结果的时候，reply信息表示节点的任期已经比现在L节点大，则表示L可能已经落后，转变身份为跟随者

如果节点选举超时，节点会转换身份为C，ticker()会进行选举工作，重复之前的选举过程。



```js
func (h *Heartbeat) IsNormal(i int) bool {
	judge := h.now > h.last

	// fmt.Printf(" index %v 调用normal now %v > last %v\n ", i, h.now, h.last)
	// fmt.Printf("now %v,last %v judge %v\n", h.now, h.last, judge)

	h.last = h.now

	return judge
}
```
#### raft 同步架构图

![2f4af53586507ea4a4308983074f4f8](https://github.com/LT0X/MIT6.824/assets/88709343/eb2ef754-cf85-4ebd-acef-8bd33d4f11df)

#### raft 后台运行的协程
![5daae2b5443f402624dc2d797a4dac2](https://github.com/LT0X/MIT6.824/assets/88709343/7a9b4d5a-a6a9-4590-a1ad-480445fb1c90)



 首先应用状态机传来请求，调用`start()` ,raft对请求进行日志化,同时进行节点日志同步，首先我这里进行的处理`start()`的快速退出，`start()`只对日志进行存储，没必要等待节点完成日志一致性才退出，这里的处理是在Lader启动的时候运行了一个后台监控协程 ` SendAppendLogsHandler()`, 监控协程通过指定周期进行轮询的方式进行监控，只要监控到leader缓存其他节点matchIndex和Leader当前节点的 rf.logLastIndex 是否相同，如果不同，表示其他节点于本leader节点日志未达成一致性，需要进行日志同步。

我这样设计的考虑是是为了提高响应速度，因为`start()`是一个高频率的一个函数，用途是用来处理接受状态机的操作日志进行同步，但是如果把日志同步也写进Start()函数里面，这样会大大影响响应性能，因为如果这样设计就等于处理一条就要进行一次同步，就会造成许多的I/O的浪费，因为同步需要是进行远程的RPC操作，需要考虑的网络的稳定和延迟，所以我这边的处理是每一个leader后台都有一个处理协程，用于监听节点是否需要进行日志同步，经过一次周朝时间做一次检测，然后批处理发送这一次周期时间里面的所有日志，这样进行了对主节点的日志增加操作和节点日志的同步进行了分离，进行了解耦，然后也提高了响应速度


```js
func (rf *Raft) Start(command interface{}) (int, int, bool) {

	...............
        
	term, isLeader = rf.GetState()
	rf.mu.Lock()
	rf.logs = append(rf.logs, &log)
	//保证数组索引和log Index 的一致性
	rf.logLastIndex = rf.lastIncludedIndex + len(rf.logs) - 1
	rf.logLastTerm = nowTerm
	index = rf.logLastIndex
	// fmt.Printf("!!!!@@@@@@@@@@ index is %v command %v\n", index, command)
	rf.mu.Unlock()

	return index, term, isLeader
}
```



```js
func (rf *Raft) SendAppendLogsHandler() {

	for true {

		time.Sleep(100 * time.Millisecond)
		if rf.state != "L" {
			return
		}
                  ....................

		// fmt.Printf("^^^^^^^^^^^L %v statate %v lastIndex is %v 时间 地址……%v \n",
		// 	rf.me, rf.state, lastIndex, &rf)
		for k := 0; k < len(rf.peers); k++ {
                
			...................
                        
			// fmt.Printf("^^^^^^^^^^^rf.index %v 节点 k %v lastIndex is %v \n",
			// 	rf.me, k, rf.matchIndex[k])
			if rf.matchIndex[k] != lastIndex {
				// logs := []*Log{}
				// logs = append(logs, rf.logs[rf.preAppendLogIndex:]...)
				// fmt.Printf("index %v 开始发送日志 %v term %v\n", rf.me, command, rf.currentTerm)
				
                                ..................
                           }
                           
                          ..............
                           
             }
             
   }          
```
如果在` SendAppendLogsHandler()`监测到了 日志不同步，进行日志发送的相关工作，进行RPC 请求args和reply的相关创建，同时为每一个需要发送的节点创建的一个协程`rf.AppendEntriesRunTime()`进行异步的请求发送和请求跟踪，


```js
                              args = AppendEntriesArgs{
					Term:         Term,
					LeaderId:     rf.me,
					PrevLogIndex: rf.nextIndex[k] - 1,
					PrevLogTerm:  rf.logs[i].Term,
					Entries:      rf.logs[index:],
					LLeft:        rf.lastIncludedIndex + 1,
					LRight:       rf.lastIncludedIndex + len(rf.logs) - 1,
					LeaderCommit: rf.commitIndex,
				}

				// fmt.Printf("&&&&&&&&&&& L index %v向 %v 发送 日志 [%v:%v]\n",
				// 	rf.me, k, nextIndex, lastIndex+1)
				reply := AppendEntriesReply{}

				rf.isAppendDone[k] = false
				go rf.AppendEntriesRunTime(k, &Done, &args, &reply)
```
 
`AppendEntriesRunTime()`是处理日志同步的关键，首先按照raft论文的说明，日志的同步是需要先找到F节点和raft的节点，两个节点同步日志的索引，也就是论文里面说的日志索引相同，同时从节点和主节点在这一条相同日志索引上面的任期号要一样，我们要找到这个匹配的最大索引，原因是raft日志有以下性质。

> Raft 维护着以下的日志匹配特性：
> 
> -   **如果在不同的日志中的两个条目拥有相同的索引和任期号，那么他们存储了相同的指令。**
> -   **如果在不同的日志中的两个条目拥有相同的索引和任期号，那么他们之前的所有日志条目也全部相同。**

  我们主节点要先找到和从节点最大的日志同步索引，然后再发送在这个索引之后主节点的日志给从节点进行应用和复制, raft给出的方法是通过RPC请求发送，然后主节点根据从节点响应调整递减nextidnex-1，
我这里对这个寻找最佳日志索引的位置进行算法的相关的优化，就是借鉴了一下算法二分的思想，首先二分的使用是需要有有序的，同时数组是需要有二分性，具有单调性，也就是有序的，同时还需要有有界，这意味着我们需要知道目标值所在的范围，这里的寻找最佳日志索引近似符合这种情况，找最佳的匹配日志索引，说白了，就是寻找L节点的logs[]和F节点logs[],里面找索引相同且索引日志的任期号也相同的最大索引值，这里为什么用了二分优化是因为， raft日志有一个上面提到的一个很重要的性质, <br>

***如果在不同的日志中的两个条目拥有相同的索引和任期号，那么他们之前的所有日志条目也全部相同***

这样的话数据就是有单调性和二分性了，如果找到了一个索引是符合的，那它前面的索引就是全部符合的，如果找到一个索引是不符合的，那它后面的也就是全部不符合的，那这样的话我们就可以用二分进行匹配的优化, 这样的话可以把匹配的算法时间复杂度从**O(n)** 优化到 **O(logn)** <br>
nextIndex[]表示L节点缓存F节点的需要的下一个日志索引，在leader选举成功后，会把自身所有的`nextIdnex[i] = rf.logLastIndex + 1`, 首先让`PrevLogIndex: rf.nextIndex[k] - 1`,让F节点进行第一次一致性验证，如果一致性验证通过则直接进行日志添加，如果一致性不通过，则需要进行最佳日志索引的寻找，寻找到最佳的时候，才进行日志添加,


```js
                               
                                //同步日志的RPC请求args
                                args = AppendEntriesArgs{
					Term:         Term,
					LeaderId:     rf.me,
					PrevLogIndex: rf.nextIndex[k] - 1,
					PrevLogTerm:  rf.logs[i].Term,
					Entries:      rf.logs[index:],
					LLeft:        rf.lastIncludedIndex + 1,
					LRight:       rf.lastIncludedIndex + len(rf.logs) - 1,
					LeaderCommit: rf.commitIndex,
				}
```

```js

                    //从节点进行一致性验证
                        if lastIndex == args.PrevLogIndex &&
			rf.logs[lastIndex].Term == args.PrevLogTerm &&
			args.Entries != nil {
			//已经到达一致，直接添加
			rf.logs = append(rf.logs, args.Entries...)
			// rf.logLastIndex = len(rf.logs) -1
			rf.logLastIndex = rf.lastIncludedIndex + len(rf.logs) - 1
			index = len(rf.logs) - 1
			rf.currentTerm = rf.logs[index].Term
			rf.logLastTerm = rf.logs[index].Term
			// fmt.Printf("<<<<<<< index %v \n", index)
			reply.Success = true
			rf.isLogCommon = true
			// fmt.Printf("1添加成功 index %v lastTerm %v loglastindex %v\n", rf.me, rf.logLastTerm, rf.logLastIndex)
			rf.persist()
			return

		}
```

进行最佳日志索引的匹配的时候，这时候需要两组数据进行比较和通信，第一组数据是L节点的raft的logs[]数据，另外一组数据是F节点是raft的logs[]，在RPC通信的时候，为了节省网络带宽,在未找到最佳日志索引的时候，RpcArgs里面的logs为nil,直到找到最佳索引的时候，args才会携带数据，<br>

这里面二分查找的时候，可以以L节点的logs[]数据为主体，然后发送L的mid数据给F节点进行比较，也可以以F为主体发送logs[]信息给L节点，我这里是以L节点logs[]为主体, 主要流程就是，L节点在RPCArgs里面添加 LLeft和LRight，表示logs的边界，同时也表示二分里面的l和r, 同时RpcArgs里面的还PrevLogIndex和 Term ,表示二分里面的mid和mid对于日志索引的任期，然后F接受到请求，开始比较自身log[]和PrevLogIndex和Term的匹配关系，匹配则继续二分改变mid值寻找最佳匹配，不匹配则同理，
直到主节点收到的回应`reply.LLeft > reply.LRight` 开始判断决定匹配结果。





```js
                       //L节点处理逻辑
                       if reply.LLeft <= reply.LRight {
			//可以继续调整
			//继续匹配
			args.LLeft = reply.LLeft
			args.LRight = reply.LRight
			mid := reply.LLeft + (reply.LRight-reply.LLeft)/2
			args.PrevLogIndex = mid
			index = mid - rf.lastIncludedIndex

			if index <= 0 {
				//nextIndex 小于快照压缩索引
				rf.lSnapshotChan <- LSnapshot{
					k: k,
				}
				return
			}

			args.PrevLogTerm = rf.logs[index].Term
			args.Entries = nil
			args.First = false

			//开始发送
			rf.sendAppendEntries(k, args, reply)

		} else {
			if reply.LRight < 0 {
				//寄了，匹配不到
				rf.isAppendDone[k] = true
				rf.nextIndex[k] = rf.lastIncludedIndex
				return
			}

			//最佳匹配，开始发送日志
			// index := rf.logLastIndex
			bestIndex := reply.LRight - rf.lastIncludedIndex
			if bestIndex <= 0 {
				//nextIndex 小于快照压缩索引
				rf.lSnapshotChan <- LSnapshot{
					k: k,
				}
				return
			}
			args.Entries = rf.logs[bestIndex+1:]
			args.PrevLogIndex = reply.LRight
			args.PrevLogTerm = rf.logs[bestIndex].Term
			args.LLeft = reply.LLeft
			args.LRight = reply.LRight
			rf.nextIndex[k] = reply.LRight + 1
			//最佳匹配，开始发送k
			rf.sendAppendEntries(k, args, reply)

		}
```







```js
                             
                             //F节点处理逻辑
                             if args.LLeft > args.LRight {
				//已经匹配索引,复制日志
				preIndex := args.PrevLogIndex - rf.lastIncludedIndex
				rf.logs = rf.logs[:preIndex+1]
				rf.logs = append(rf.logs, args.Entries...)
				// rf.logLastIndex = len(rf.logs) - 1
				index = len(rf.logs) - 1
				rf.logLastIndex = rf.lastIncludedIndex + len(rf.logs) - 1
				rf.currentTerm = rf.logs[index].Term
				rf.logLastTerm = rf.logs[index].Term
				reply.Success = true
				rf.isLogCommon = true
				rf.persist()
				return

			}

			if args.PrevLogIndex-rf.lastIncludedIndex <= 0 || rf.logs[args.PrevLogIndex-rf.lastIncludedIndex].Term == args.PrevLogTerm {
				//通过，继续尝试寻找
				reply.IsOk = true
				reply.LLeft = args.PrevLogIndex + 1
				reply.LRight = args.LRight
				reply.LRight)
				return
			} else {
				//不通过，
				reply.IsOk = false
				reply.LLeft = args.LLeft
				reply.LRight = args.PrevLogIndex - 1
				return
			}
```
成功完成日志复制的节点如果达到了“大多数”（完成节点的数量 m >= n/2+1）,要进行提交操作，L节点告知状态机请求已经完成，F节点则需要进行把日志操作应用到状态机,我前面把`start() `添加日志和向其他节点复制做了一个解耦分离，独立出了两个处理流程，同样提交日志也做了一个处理分离， 首先当节点选举成为leader的时候，它会启动一个后台提交日志协程`LCommitLogRunTime()`，F启动的时候则会启动一个F节点提交日志的协程` FCommitLogRunTime()`。

`LCommitLogRunTime()`后台协程会接受上面提到进行日志复制的`AppendEntriesRunTime()`的相关信息，`AppendEntriesRunTime()`会对节点复制的处理进行检测和重试，如果成功的话，`AppendEntriesRunTime()`会对`LCommitLogRunTime()`发送成功日志复制的日志相关信息，然后`LCommitLogRunTime()`会在监控协程里面维护一个commitMap， 里面对`AppendEntriesRunTime()`发来的成功同步日志的索引信息进行统计，然后自增1，如`commitMap[v.AppendIndex]++`，然后会对这个大小进行判断，如果大小大于等于全部节点的“大多数”(n/2+1)，则会把日志提交到状态机，通知状态机操作日志已经完成一致性 。


```js
     func (rf *Raft) AppendEntriesRunTime(k int, Done *int32,
	args *AppendEntriesArgs, reply *AppendEntriesReply) {
        
        
          .............
          .............
          
         //k节点同步成功，向LCommitLogRunTime()发送管道信息
         rf.lCommitChan <- AppendDone{
		k:           k,
		AppendIndex: rf.matchIndex[k],
		IsExit:      false,
	}
	rf.mu.Lock()
	rf.isAppendDone[k] = true
	rf.mu.Unlock()
}
```


```js
             
    func (rf *Raft) LCommitLogRunTime() {

	
	commitMap := make(map[int]int, 0)

	for true {

		select {
		case v, ok := <-rf.lCommitChan:
			if ok && !v.IsExit {

				if v.AppendIndex <= rf.commitIndex {
					//已经提交了，不做处理
					// fmt.Println("已经提交，******")
					continue
				} else 
					commitMap[v.AppendIndex]++
					index := commitMap[v.AppendIndex]
					
					if index >= len(rf.peers)/2 {
						//到达提交条件,开始提交
						for i := rf.commitIndex + 1; i <= v.AppendIndex; i++ {
                                                ..........
                                                x := ApplyMsg{
								CommandValid: true,
								Command:      commad,
								CommandIndex: i,
							}
                                                 
                                                .........
                                                 
						rf.applyCh <- x

						}
						..........
					}
				}
			} else {
				return
			}
		}
	}
}
```


`FCommitLogRunTime()`F节点什么时候提交应用日志操作到状态机是取决于Leader节点，要等到Leader成功提交后，leader会把自己最新成功提交的日志索引信息携带在心跳请求的Args中，然后随着心跳信息的发送，F节点在心跳的处理流程中进行判断，如果大于自身最大的提交索引，则表示自己要更新提交，这时就会向`FCommitLogRunTime()`发送相关信息，提醒raft把日志操作应用到状态机中，`FCommitLogRunTime()` 接受到信息，会进行一些相关的安全性判断，然后提交操作日志，把操作应用到状态机中。


```js
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	.............
	if args.Entries == nil && args.LLeft == -1 {

		//表示心跳
		if args.Term >= rf.currentTerm && args.LeaderId == int(rf.votedFor) {
			
                        ....................
			
			if args.LeaderCommit > rf.commitIndex && args.PrevLogIndex != 0 {
				// fmt.Printf("rf.me %v 启动 提交线程 argsIndex %v\n", rf.me, args.PrevLogIndex)
				rf.fCommitChan <- FCommit{
					CommitIndex:  args.LeaderCommit,
					PrevLogIndex: args.PrevLogIndex,
					PrevLogTerm:  args.PrevLogTerm,
				}
			}
			return
		}
		reply.Success = false
		return
	}
        ................
 }       
```


```js

func (rf *Raft) FCommitLogRunTime() {

	for true {

		select {
		case v, ok := <-rf.fCommitChan:
				//准备提交
				if v.PrevLogIndex-rf.lastIncludedIndex < 0 {
					// fmt.Printf("索引超出，数组溢出*********************")
					continue
				}

				if v.PrevLogIndex == 0 ||
					v.PrevLogIndex > rf.logLastIndex ||
					rf.logs[v.PrevLogIndex-rf.lastIncludedIndex].Term != v.PrevLogTerm {	
					continue
				}

				
				..............
				
				for i := lastCommitIndex; i <= int(min); i++ {
                                
					.............
                                        
					msg := ApplyMsg{
						CommandValid: true,
						Command:      commad,
						CommandIndex: i,
					}
					...............
					rf.applyCh <- msg
					rf.commitIndex = i

				}

			}

		}
	}
}
```



接下来是一个关于raft自身状态的持久化的一个处理，这一方面，我的策略是只要我需要持久化的raft变量进行改变就进行一次持久化，所以关键就是确定持久化的raft变量，以便下次raft重启的时候进行读取raft状态持久化文件，进行快速的一个的启动，我想我进行处理的时候比较简单粗暴，我这里应该还有许多可以优化的选项，


```js

func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.state)
	e.Encode(rf.logs)
	e.Encode(rf.preCommitIndex)
	e.Encode(rf.commitIndex)
	e.Encode(rf.nextIndex)
	e.Encode(rf.preAppendLogIndex)
	e.Encode(rf.isLogCommon)
	e.Encode(rf.logLastIndex)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)

	raftstate := w.Bytes()

	// fmt.Printf("raft %v 状态大小为 %v loglen %v\n", rf.me, len(raftstate), len(rf.logs))
	rf.persister.Save(raftstate, rf.persister.snapshot)
}
```

接下来就是raft日志压缩的处理，由于是raft是处理上层状态机发来的操作进行日志化，所以随着时间的推移，raft的日志会越来越长，但是随着时间的推移，上层状态机的数据也在不断更新，所以这些之前的操作日志也逐渐失去时效性，所以当上层状态机检测到raft的状态占用的大小将要超过一个阈值，就会发送一个日志压缩请求给raft提醒进行日志压缩工作，主要的处理流程的是，对压缩索引后面的日志进行保存，压缩索引的前面的日志进行全部的丢弃，然后更新`raft.lastIncludedIndex`（表示节点最新的压缩索引），往后的日志操作都依靠`raft.lastIncludedIndex`对真实日志索引和rf.logs[]的存储索引进行计算来映射，


```js
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	//开始截取，获取锁，防止和主节点添加日志发生并发错误
	rf.mu.Lock()
	rf.persister.snapshot = snapshot
	lastIndex := index + 1 - rf.lastIncludedIndex
	rf.lastIncludedIndex = index
	
	rf.lastIncludedTerm = rf.logs[rf.logLastIndex-rf.lastIncludedIndex].Term
	if rf.logLastIndex-rf.lastIncludedIndex == 0 {
		rf.lastIncludedTerm = rf.logLastTerm
	}
	temp := rf.logs[lastIndex:]
	rf.logs = rf.logs[0:1]
	rf.logs = append(rf.logs, temp...)
	
	rf.persist()
	rf.mu.Unlock()

}
```
同时还需要注意的另一个问题，如果一个落后于leader的一个F节点，如果它想要的日志或者和Leader同步的最佳日志索引已经被Leader日志压缩丢弃，则这个时候，leader就得发送一个状态机的快照给F节点，使它同步于自己的状态，认它把最新的快照应用到自己的状态机机，同时对日志进行压缩，但是有一个要注意的点，<br>
“**快照所覆盖的日志条目将被删除，但快照之后的条目仍然有效，必须保留，因为这些条目可能包含了一些还没有被应用到状态机的命令，或者一些还没有被其他节点复制的命令**” <br>
所以在进行接受快照更新后的日志压缩操作的时候，不仅要更新自己的lastIncludedIndex和lastIncludedTerm，这两个变量指的是被经过快照后，被修剪的最后一个日志的索引和任期值，除了要压缩丢弃日志到lastIncludeIndex这个索引值，同时还要保存快照索引之后的日志条目。



```js

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	//定位3,节点nextIndex 日志已经压缩,进行快照复制

	if args.Term < rf.currentTerm || args.LeaderId != int(rf.votedFor) ||
		args.LastIncludedIndex < rf.lastIncludedIndex {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	if rf.logLastIndex > args.LastIncludedIndex {
		//重新传输或错误操作
		//快照所覆盖的日志条目将被删除，但快照之后的条目仍然有效，必须保留。
		index := args.LastIncludedIndex - rf.lastIncludedIndex
		temp := rf.logs[index+1:]
		rf.logs = rf.logs[0:1]
		rf.logs = append(rf.logs, temp...)
		rf.lastIncludedIndex = args.LastIncludedIndex
		rf.lastIncludedTerm = args.LastIncludedTerm
		rf.persister.snapshot = args.Data
		rf.persist()

	} else {
		//日志清空,

		rf.mu.Lock()

		// fmt.Printf("日志清空准备 %v argsLastIndex  %v rfLast %v\n",
		// 	rf.me, args.LastIncludedIndex, rf.logLastIndex)
		rf.logLastIndex = args.LastIncludedIndex
		rf.logLastTerm = args.LastIncludedTerm
		rf.lastIncludedIndex = args.LastIncludedIndex
		rf.lastIncludedTerm = args.LastIncludedTerm
		rf.logs = rf.logs[0:1]

		rf.mu.Unlock()

		//开始应用到状态机
		rf.applyCh <- ApplyMsg{
			CommandValid:  false,
			SnapshotValid: true,
			Snapshot:      args.Data,
			SnapshotTerm:  int(rf.lastIncludedTerm),
			SnapshotIndex: rf.lastIncludedIndex,
		}
		// fmt.Printf("&&&&&&&&&&&&&&&&被应用到状态机了 %v\n", rf.me)
		rf.commitIndex = args.LastIncludedIndex

		rf.persister.snapshot = args.Data
		rf.persist()
	}

}
```
这个就是我整体的raft的架构设计，当然还有一些同步和互斥的并发访问设计，注意并发安全的访问，注意死锁的避免，其他细节问题就不展开了。

<br>
<br>
<br>


## Lab3 Fault-tolerant Key/Value Service

### 整体的架构设计


#### KVService架构设计图
![9f32f8bb6adcf0891f4e9c51d33d960](https://github.com/LT0X/MIT6.824/assets/88709343/692457d2-9f7f-4017-a1b3-9675d3a502c6)
#### KVService 后台运行的协程
![db73be0d32fd396a6f6bb51bac8c5fd](https://github.com/LT0X/MIT6.824/assets/88709343/3196db7f-8730-4983-b4f9-2146713ca880)


### raft的技术细节

这次的Lab是和之前做的Lab2 raft进行结合做一个可靠的KVService，要实现整体的架构是C/S架构，即客户端和服务器的形式，实现的主体有Client 和Server.

首先就是客户端的实现，客户端要实现的主要是两个RPC函数，一个是PutAppend(),主要用来添加Key/value和对key的值进行更新， 另外一个是Get()，用来获取key的值<br>

所以客户端Client实现的主体主要进行发送请求和接受服务器的响应，无论是PutAppend()还是Get()，在客户端的实现都是发送请求，其中实现的时候是有许多共同实现的点，首先，客户端是并不直到那个是主Server,因为只有leader才可以处理请求，也就是说，客户端并不知道那个Server的raft是leader，所以这个时候，是需要向多个server的进行发送请求，直到找到可以处理请求的server，我这里的处理开始发送请求的时候，是调用里面的 `nrand()`函数，其返回的随机数对Server数量取余得到index，如果身份不符合leader,则对index进行`(ck.leaderIndex + 1) % len(ck.servers) `，如果找到了leader,则对其Leader Server进行缓存，节省下次发送请求寻找leader Server的时间



```js

func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.

	args := PutAppendArgs{
		Key:   key,
		Value: value,
		Op:    op,
		Id:    int(atomic.AddInt32(&opId, 1)),
	}
	reply := PutAppendReply{
		NotLeader: false,
	}
	
	for !reply.Done {

		if reply.NotLeader {
			ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)

		}
	
		ok := ck.sendPutAppendRPC(ck.leaderIndex, &args, &reply)

		if !ok {
			// fmt.Printf("客户端发起RPC失败 opid %v,reply %v,cklindex %v\n",
			// 	args.Id, reply, ck.leaderIndex)

			reply.NotLeader = true
			
		}

	}

	

}
```

为了唯一标识每一个请求，在客户端发送请求的时候，都会得到一个opId，获取ID的方式是原子操作，确保每一个op都拥有一个唯一不重复的ID,同时客户端还应该有重试的机制，不仅是为了应对寻找Leader的多次RPC请求，同时也是为了应该各种网络问题和分区问题，这里的重试机制主要实现是通过循环对reply.Doen 判断来实现的，当服务端正常的处理请求都会为`reply.Done = true`进行赋值，以此来表示请求的正常完成


```js

func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.

	args := GetArgs{

		Key: key,
		Id:  int(atomic.AddInt32(&opId, 1)),
	}
	reply := GetReply{
		NotLeader: false,
	}
	
	for !reply.Done {

		if reply.NotLeader {
			ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)

		}

	
		ok := ck.sendGetRPC(ck.leaderIndex, &args, &reply)

		if !ok {
			// fmt.Printf("客户端发起RPC失败 opid %v,reply %v,cklindex %v\n",
			// 	args.Id, reply, ck.leaderIndex)
                        
			reply.NotLeader = true
			
		}

	}

	return reply.Value
}
```
&nbsp;&nbsp; 接下来就是Server的实现,Server的主要用途是键值对服务器，接受客户端Client  Get()和PutAppend()的请求，然后把操作日志传输到raft,然后让日志进行一致性同步，完成一致性以后再对服务器本地维护的键值对进行更新，<br>
&nbsp;&nbsp;在设计Server的时候每一个Server除了维护本地的KvMap，还要维护一个管道池channelPool，opidMap,clientMap,

**opidMap** 的设计是为了唯一标识一个操作是否已经被KVserver处理，防止重复的处理，因为可能出现网络通信的问题，导致server回复的RPC请求客户端client并没有收到，然后客户端client重新发送相同的请求，这个时候就得靠opidMap判断server是否已经处理了请求<br>

**channelPool** 的设计是为了适应之前raft的批处理提交，提交操作的日志，然后同步日志和提交日志到状态机，这一系列操作并不是线性同步完成的，是异步完成的，所以这个时候有一个异步的等待时间，主要时间再于raft函数`SendAppendLogsHandler()`的轮询时间,因为是处理是基于批处理操作，其他操作都是快进快出，所以主要的性能瓶颈在轮询时间可以理解，这个由于测试的时候，性能要求是平均33ms每一个请求，所以我把之前的轮询时间调整成了大概20ms每轮询一次，这个时间我个人觉得有点轮询时间太短了，对CPU的性能可能造成一定的浪费，但是我又想到另外一个思路，就是动态确定一个轮询时间，当请求高并发的时候缩短轮询时间，当请求并发低的时候可以增大轮询时间，但是我考虑到一个弊端就是，当请求少的时候，反而处理时间要长，因为主要的性能瓶颈就在轮询时间，而且MIT6.824的测试用例就是高并发满负荷运行的，所以索性直接调整转轮询时间为适应测试大小，所以说回来了为什么要用到这个**ChannelPool**,它池里面的对象是golang里面的管道技术，协程可以通过管道发送数据信息，通过对管道的监控可以达到同步协程或者是通知的作用，所以我用它来监控raft是否已经日志同步成功，以此来向客户端client返回响应信息。后面这里的处理我会展开讲

**clientMap**的作用是用来表示是否和客户端client保持一个连接，因为这里的通信涉及到一个管道通信，如果不能确定客户端是否还保持连接，kvservice可能会在一些异常情况向一个没有被监听的管道发送信息，而导致这个管道里面的信息没有被处理而导致整个进程kvservice的阻塞，



```js

type KVServer struct {
	mu           sync.Mutex
	me           int
	rf           *raft.Raft
	applyCh      chan raft.ApplyMsg
	snapshotCh   chan raft.ApplyMsg
	kvMap        map[string]string //本地service键值对
	channelPool  ChannelPool       //管道池
	opIdMap      map[int]bool      //表示本地service是否已经处理该编号的请求
	clientMap    map[int]bool      //表示客户端是否还和Server链接
	snapshotDone bool              //表示kvServer创建快照任务是否已经完成

	persister *raft.Persister

	lastOpId int   //处理请求的 最后的opId
	dead     int32 // set by Kill()

	rwLock sync.RWMutex

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
}
```

接下来就是来说明一下设计的kvService的处理架构设计，首先为了可以能够给客户端client进行RPC调用，同样server方面的处理函数也是PutAppend()和Get()。同样他们也有许多相同的处理流程。

首先来说一下 PutAppend()的完成处理一个请求的流程,首先会先检测opidMap来确定一个请求是否已经被处理，未被处理则会调用raft的`start()`进行日志的添加，然后会在管道池中根据opid进行分配一个管道连接，然后会对此管道进行一个监控，<br>

这里说一下管道池的具体的设计细节，首先为什么要用到管道池除了上面说明的原因，还有另外一个考虑就是性能的问题，因为对每一个请求都要创建一个管道来检测请求是否已经完成，频繁的管道创建和管道回收我觉得是一种性能的浪费，同时管道我只用来监控请求，这是一种可以频繁重复利用的一种资源，所以结合情况，就想到了池化技术，想要管道的时候去里面根据opid哈希获得一个管道，后续其他协程也可以根据opid从管道池中获取这个opid对应的管道来发送信息，以此来达到重复利用和通信，然后就是从管道池怎么根据opid获取id，为了尽可能避免哈希冲突，和为了每一个管道可以充分的利用，最初的设计的方案是通过golang 里面自带哈希算法来根据opid来进行离散化获取管道索引，但是后来发现我是根据opid来获取管道索引，同时我的opid每次都是单调递增1的，我可以对opid和管道池的数量进行取模操作，这样每一个管道都可以很紧凑的运用到，同时哈希冲突的次数也会减少，因为只有最后一个管道分配出去，第一个管道才会重新会分配。







```js
// ChannelPool is a struct that represents a pool of boolean channels
type ChannelPool struct {
	channels []chan int // the slice of channels
	size     int        // the size of the pool
}

// NewChannelPool creates a new ChannelPool with the given size
func NewChannelPool(size int) *ChannelPool {
	// create a slice of channels with the given size
	channels := make([]chan int, size)
	// initialize each channel
	for i := range channels {
		channels[i] = make(chan int, 2)
	}
	// return a pointer to the ChannelPool
	return &ChannelPool{
		channels: channels,
		size:     size,
	}
}

// GetChannel returns a channel from the pool based on a hash of an int32 value
func (cp *ChannelPool) GetChannel(value int32) chan int {
	// create a new hash function
	// h := fnv.New32a()
	// write the value as bytes to the hash function
	// h.Write([]byte(fmt.Sprint(value)))
	// get the hash value as an uint32
	// hash := h.Sum32()
	// get the index of the channel by modding the hash with the size of the pool
	index := int(value) % cp.size
	// return the channel at the index
	return cp.channels[index]
}

```

为请求分配一个管道之后，就开始对此管道进行监控，之后就等其他的协程对此管道发送结束请求，
在每一个KvService 进行初始化的时候，都会启动两个后台的协程，一个是`kv.snapshotRunTime()`,这个是监控快照相关处理，这个后面说明,另外一个就是`kv.applyChRunTime()`,也就是为现在要说明，这个后台协程的作用是来接受raft提交过来的日志请求，raft提交的日志都说明已经成功q完成一致性的同步，可以进行状态机的应用，接受到的信息的时候，如果Server的raft是leader节点，则根据raft提交的操作日志的opid从管道池获取管道，同时根据opid来看一下clientMap，服务器和客户端是否已经失去连接，防止管道信息没有处理得到阻塞，开始向管道发送结束请求，同时将日志操作应用到本地KvService,只有`applyChRunTime()`提交的日志才能应用，并不会直接在`PutAppend()`函数进行状态更新，同时`applyChRunTime()`的作用还有接受raft传来的快照，然后进行自己状态更新

```js

func (kv *KVServer) applyChRunTime() {

	for true {
		select {

		case msg, _ := <-kv.applyCh:

			if op, ok := msg.Command.(Op); ok && msg.CommandValid {
				
				if _, ok := kv.rf.GetState(); ok {
					c := kv.channelPool.GetChannel(int32(op.Id))
					_, ok := kv.rf.GetState()
					
					

					v, ok := kv.getSyncMap(op.Id, kv.clientMap)

					if v && ok {
                                        //发送请求结束通知
						c <- op.Id
					}

					
				}

				if kv.maxraftstate != -1 && kv.snapshotDone && msg.IsLast &&
					float32(kv.persister.RaftStateSize()) > (float32(kv.maxraftstate)*0.8) {
					//日志过长，需要进行日志压缩持久化
                                        
					kv.snapshotCh <- msg

				}

				if v, _ := kv.getSyncMap(op.Id, kv.opIdMap); v {
					
					continue
				}

				kv.executeOp(op)
				kv.setSyncMap(op.Id, true, kv.opIdMap)
				

			} else if msg.SnapshotValid {
				kv.DecodingKvStatus(msg.Snapshot)
			}

		}

	}
}
```

PutAppend select 到`applyChRunTime()`的结束的信息以后，就会进行结束的相关的处理，进行RPC回复客户端client,同时进行客户端断开连接的标记，同时每一个请求的管道检测都有时间限制，如果一定的时间内Server 检测的管道没有检测到信息，则表示这个server在规定时间内，raft未能达成一致性，这个server可能不具备处理请求能力，会强制回复请求失败的信息到客户端并进行断开链接标记，并同时告知客户端重新寻找Leader server，更新leader server缓存，


```js

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	

	if v, ok := kv.getSyncMap(args.Id, kv.opIdMap); v && ok {
		reply.Done = true
		return
	}
	op := Op{
		Optype: args.Op,
		Key:    args.Key,
		Value:  args.Value,
		Id:     args.Id,
	}

	_, _, isLeader := kv.rf.Start(op)

	
	if !isLeader {
		//不是leader节点，需要重新返回重试
		reply.NotLeader = true

		return
	}

	//表示客户端正在于服务器链接

	kv.setSyncMap(args.Id, true, kv.clientMap)

	defer func() {
		kv.setSyncMap(args.Id, false, kv.clientMap)
	}()

	// fmt.Printf("管道开始监听 opid %v me %v \n", args.Id, kv.me)
	c := kv.channelPool.GetChannel(int32(args.Id))

	for true {

		select {

		case isok, _ := <-c:
			
			if isok == args.Id {
				// fmt.Printf("管道准备释放 opid %v me %v\n", args.Id, kv.me)
				reply.Done = true
				reply.ServerLastOpId = kv.lastOpId
				
				return
			} else {
				continue
			}
			
		case <-time.After(3 * time.Second):
			// fmt.Printf("监管函数 强制下线 opid %v me %v\n", args.Id, kv.me)
			reply.NotLeader = true
			return
		}

	}


}
```
Get()函数处理流程大同小异，只是PutAppend()是对server Kv进行更新，而Get()是对server的key 进行取值，这里就不再重复



```js

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.

	op := Op{
		Optype: "Get",
		Key:    args.Key,
		Id:     args.Id,
	}

	_, isLeader := kv.rf.GetState()

	if v, ok := kv.getSyncMap(args.Id, kv.opIdMap); isLeader && v && ok {
		//已经存在，提前返回
		reply.Done = true
		v, IsExit := kv.getMapValue(args.Key)
	        .............
		reply.Value = v
		return
	}

	_, _, isLeader = kv.rf.Start(op)


	if !isLeader {
		if !isLeader {
			//不是leader节点，需要重新返回重试
			reply.NotLeader = true

			return
		}
	}

	//表示客户端正在于服务器链接

	
	kv.setSyncMap(args.Id, true, kv.clientMap)

	defer func() {
	
		kv.setSyncMap(args.Id, false, kv.clientMap)
	}()


	c := kv.channelPool.GetChannel(int32(args.Id))
	

	for true {

		select {
		case isok, _ := <-c:

			if isok == args.Id {

				
				v, IsExit := kv.getMapValue(args.Key)
				reply.Done = true
				reply.ServerLastOpId = kv.lastOpId
				// fmt.Printf("管道准备释放 opid %v \n", args.Id)
				if !IsExit {
					reply.Value = ""

					return
				}

				reply.Value = v
				
				return
			} else {
				continue
			}
	
		case <-time.After(3 * time.Second):
			// 3s服务器未能处理，表示出现异常，返回false
			// fmt.Printf("监管函数 强制下线 opid %v me %v\n", args.Id, kv.me)
			reply.NotLeader = true
			return
		}

	}

}
```
接下来就是关于对server进行一个快照操作,之前的说过，server启动的时候，会启动后台协程`kv.snapshotRunTime()` 这个后台的协程作用是，对server的raft的日志大小进行监控，如果raft的状态快照将要达到的阈值（maxraftstate），则会发送一个日志压缩请求到raft进行raft的日志压缩操作，同时server自身也会对自己的状态进行编码，然后做一个持久化快照操作，在下次server重新启动的时候，会读取这个状态快照文件，达到一个快速启动的效果



```js
func (kv *KVServer) snapshotRunTime() {
	//用于监控raft节点是否日志过长

	for true {

		select {
		case v, _ := <-kv.snapshotCh:

			kv.snapshotDone = false

			// kv.rf.CompressLogHandler(v.CommandIndex)
			//进行kvservice 的编码

			// start := time.Now()

			data := kv.EncodingKvStatus()

			kv.rf.Snapshot(v.CommandIndex, data)

			kv.snapshotDone = true
			// dur := time.Since(start)
			// fmt.Printf(" 快照完成 commandIndex %v\n", v.CommandIndex)

		}
	}

}
```

**这些大概就是我这次全部完成的Lab1,Lab2,Lab3,的各种设计，虽然debug的过程是很痛苦的，但还是学到了很多。最后希望自己可以继续精进技术，成为一名优秀的技术工程师，技术极客**


## 通过测试用例截图

### MapReduce

![86f582df625cff2a0ad38d630649af9](https://github.com/LT0X/MIT6.824/assets/88709343/2097e18e-ad99-459e-8c46-a66626656fd4)

![c49fc674ca338b664581d0ea8fea6d6](https://github.com/LT0X/MIT6.824/assets/88709343/b2810717-76ec-45b3-a666-2b92e0f0502f)



![9b2d88a2fbba4e3b466ef2eb8278590](https://github.com/LT0X/MIT6.824/assets/88709343/3d3588c2-ffde-4a41-8750-8366b6180a47)

![161c059bc7b675f90ba78531dfe5f85](https://github.com/LT0X/MIT6.824/assets/88709343/e418dcd8-105d-4b88-af5d-943182b603a0)

![5437714ea5ee71e75f6bb3469ee5f6c](https://github.com/LT0X/MIT6.824/assets/88709343/debe92ef-385c-435b-b75c-aaa7d8b9d4d4)

![bd642d1620b66bd47f39f6b9dcc60c5](https://github.com/LT0X/MIT6.824/assets/88709343/d1fd449d-b894-4bcc-bf50-130c1744d465)

![376636d8d80a10b8088de26eaf11f8e](https://github.com/LT0X/MIT6.824/assets/88709343/cb9d70c8-b69f-4ccb-9259-79509d9ce99e)


##### 2A

![59747ac08c81b7b6ba778f671da1cb5](https://github.com/LT0X/MIT6.824/assets/88709343/d62772c1-e38e-49d2-aa1d-05dde0d0db05)


##### 2B

![b2ae88a7a5df75194c3147205866cec](https://github.com/LT0X/MIT6.824/assets/88709343/8b5d1f2f-965e-44eb-99a1-969ab762a601)

![d5432ee5c782d2cb40e0355a3e9de3a](https://github.com/LT0X/MIT6.824/assets/88709343/a925b575-ef84-4b4f-bb8d-bf91428c60ff)

![b8d1d9b622133978a75a383bc9eba89](https://github.com/LT0X/MIT6.824/assets/88709343/0932a654-0e20-4a45-8f88-494fa27253fc)

![fb65980bf7b904c425fe710e2b2f467](https://github.com/LT0X/MIT6.824/assets/88709343/0cf83558-54b0-420e-aba6-8437b0835801)

### Raft
##### 2C

![9790e84b5cb01859ecc9ee3003c611d](https://github.com/LT0X/MIT6.824/assets/88709343/93eabe41-3252-4e6a-87f1-477bcfd4ac67)

![fda256bd0a5fc5c1085f720ce529704](https://github.com/LT0X/MIT6.824/assets/88709343/0c8e1542-3d27-46ce-a741-4330325bca7e)

![ebd45d241f4813478ea0369e64f144f](https://github.com/LT0X/MIT6.824/assets/88709343/3f36a60f-ab22-4d8c-a216-996fdace8e9d)

![98be6c66c4b3dae0222152895926cc1](https://github.com/LT0X/MIT6.824/assets/88709343/48ec3fb3-ac3d-4c94-acdd-5375354aeac1)

![4e6dd80f34bea785affc6d4454e3ce2](https://github.com/LT0X/MIT6.824/assets/88709343/da516665-b9b1-45b9-a739-1d3c0ca855b5)

![b3ce6faa8bbfa1c4505190f33f46c35](https://github.com/LT0X/MIT6.824/assets/88709343/bf8ce328-ece2-4c19-812b-6f4d76988aec)

![f7e8ea3b56055907c90f8ab5ee4be37](https://github.com/LT0X/MIT6.824/assets/88709343/826439c7-42f8-4b9b-80aa-a126ab270d5f)

![73741a1d555c400b46485972d27d837](https://github.com/LT0X/MIT6.824/assets/88709343/592fb196-3aaa-4713-bf6c-b9b2bd1bb91f)


##### 2D

![d4d2117bec0572212a239a2abfa2c5b](https://github.com/LT0X/MIT6.824/assets/88709343/f3280cad-4e5a-40c8-bd1e-6f6597002cbd)

### KVService


#### 3A


![c0698dc2243417e2d547151c897947e](https://github.com/LT0X/MIT6.824/assets/88709343/3f7ec584-19ab-497e-aee9-b6468ff6945b)

#### 3B

![fadf83b11c8d6e8f69d661cc15d73b0](https://github.com/LT0X/MIT6.824/assets/88709343/8416acad-617b-4704-aa8a-1420bb6e4d16)
