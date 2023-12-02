package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"bytes"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labgob"
	"6.5840/labrpc"
)

var Term int32

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
	IsLast       bool

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu                sync.Mutex          // Lock to protect shared access to this peer's state
	peers             []*labrpc.ClientEnd // RPC end points of all peers
	persister         *Persister          // Object to hold this peer's persisted state
	me                int                 // this peer's index into peers[]
	dead              int32               // set by Kill()
	currentTerm       int32               //表示任期
	votedFor          int32               //表示支持的领导者编号
	voteNum           int32               //获得支持的数量
	voteTerm          int32               //表示已经投票过的任期
	state             string              //表示节点的工作专态
	heartbeat         Heartbeat           //是否收到心跳检测
	applyCh           chan ApplyMsg       //节点传输信息通道
	logs              []*Log              //表示日志列表
	logLastIndex      int                 //表示日志列表最后的索引
	logLastTerm       int32               //表示日志列表最后一条的任期
	commitIndex       int                 //表示提交日志最后的索引
	commitDone        bool                //表示判断是否有commit任务正在运行
	appendEntriesDone bool                //表示判断日志复制任务是否正在运行
	preAppendLogIndex int                 //表示上一次复制日志的索引
	preCommitIndex    int                 //表示之前提交的日志索引
	matchIndex        []int               //领导者维护跟随者最高匹配的索引
	nextIndex         []int               //领导者维护跟随者的next数组
	isLogCommon       bool                //节点是否成功保持一致

	isAppendDone  map[int]bool    //表示添加日志任务和添加快照是否完成
	fCommitChan   chan FCommit    //节点监听提交任务管道
	lCommitChan   chan AppendDone //主节点监听复制日志任务管道
	lSnapshotChan chan LSnapshot  //主节点监听是否需要快照

	lastIncludedIndex int   //快照日志最后的索引
	lastIncludedTerm  int32 //快照日志最后的任期

	// voteDoneNum int32               //表示已经完成投票的
	// failureCount      int                 //节点选举失败次数

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
}

type Log struct {
	Command interface{}
	Term    int32
	// Index   int
}

type Heartbeat struct {
	last int
	now  int
}

type AppendDone struct {
	k           int
	AppendIndex int
	IsExit      bool
}

type FCommit struct {
	CommitIndex  int
	PrevLogIndex int
	PrevLogTerm  int32
}

type LSnapshot struct {
	k      int
	IsExit bool
}

type AppendEntriesArgs struct {
	Term         int32
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int32
	Entries      []*Log
	LeaderCommit int
	First        bool
	LLeft        int
	LRight       int
}

type AppendEntriesReply struct {
	Term    int32
	LLeft   int
	LRight  int
	IsOk    bool //表示节点是否长度为零
	Success bool
}

type InstallSnapshotArgs struct {
	Term              int32
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int32
	Offset            int
	Data              []byte
	Done              bool
}

type InstallSnapshotReply struct {
	Term    int32
	Success bool
}

func (h *Heartbeat) IsNormal(i int) bool {
	judge := h.now > h.last

	// fmt.Printf(" index %v 调用normal now %v > last %v\n ", i, h.now, h.last)
	// fmt.Printf("now %v,last %v judge %v\n", h.now, h.last, judge)

	h.last = h.now

	return judge
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	term = int(rf.currentTerm)
	isleader = rf.state == "L"
	return term, isleader
}

func (rf *Raft) GetVoteFor() int {
	return int(rf.votedFor)
}

func (rf *Raft) GetRaftMe() int {
	return rf.me
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
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

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {

	if data == nil || len(data) < 1 {
		// bootstrap without any state?
		return
	}

	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
	// fmt.Println("读取日志开始")
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var votedFor int32
	var currentTerm int32
	var logs []*Log
	var state string
	var preCommitIndex int
	var commitIndex int
	var nextIndex []int
	var preAppendLogIndex int
	var isLogCommon bool
	var logLastIndex int
	var lastIncludedIndex int
	var lastIncludedTerm int32

	// fmt.Println("开始解码++++++++++++++++")
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&state) != nil ||
		d.Decode(&logs) != nil ||
		d.Decode(&preCommitIndex) != nil ||
		d.Decode(&commitIndex) != nil ||
		d.Decode(&nextIndex) != nil ||
		d.Decode(&preAppendLogIndex) != nil ||
		d.Decode(&isLogCommon) != nil ||
		d.Decode(&logLastIndex) != nil ||
		d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&lastIncludedTerm) != nil {

		// fmt.Println("错误！！！！！！！！！！！！！！！！！！！-")
		return

	} else {
		// fmt.Println("解码成功-----------------")
		rf.votedFor = votedFor
		rf.currentTerm = currentTerm
		rf.logs = logs
		rf.state = state
		// rf.commitIndex = commitIndex
		rf.commitIndex = commitIndex
		if lastIncludedIndex >= 0 {
			rf.commitIndex = lastIncludedIndex
		}
		rf.preCommitIndex = preCommitIndex
		rf.nextIndex = nextIndex
		rf.preAppendLogIndex = preAppendLogIndex
		rf.isLogCommon = isLogCommon
		rf.logLastIndex = logLastIndex
		rf.lastIncludedIndex = lastIncludedIndex
		rf.lastIncludedTerm = lastIncludedTerm

		// fmt.Println("{}{}votedFor:", votedFor)
		// fmt.Println("{}{}currentTerm:", currentTerm)
		// fmt.Println("{}{}logs:", logs)
		// fmt.Println("{}state:", state)
		// fmt.Println("{}preCommitIndex:", preCommitIndex)
		// fmt.Println("{}commitIndex:", commitIndex)

		rf.persist()
	}
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	//开始截取，获取锁，防止和主节点添加日志发生并发错误
	rf.mu.Lock()
	rf.persister.snapshot = snapshot
	lastIndex := index + 1 - rf.lastIncludedIndex
	rf.lastIncludedIndex = index
	// fmt.Printf("开始rf %v节点 logLastIndex %v lastIncludeIndex %v len log %v index %v\n",
	// 	rf.me, rf.logLastIndex, rf.lastIncludedIndex, len(rf.logs), index)
	rf.lastIncludedTerm = rf.logs[rf.logLastIndex-rf.lastIncludedIndex].Term
	if rf.logLastIndex-rf.lastIncludedIndex == 0 {
		rf.lastIncludedTerm = rf.logLastTerm
	}
	temp := rf.logs[lastIndex:]
	rf.logs = rf.logs[0:1]
	rf.logs = append(rf.logs, temp...)
	// fmt.Printf("结束rf %v节点 logLastIndex %v lastIncludeIndex %v templen %v log %v index %v\n",
	// 	rf.me, rf.logLastIndex, rf.lastIncludedIndex, len(temp), len(rf.logs), index)
	rf.persist()
	rf.mu.Unlock()
	// fmt.Printf("$$$$$ 快照!!!!!!!!!!! index is  %v lastInclue %v rf.index %v+++++++++\n",
	// 	index, rf.lastIncludedIndex, rf.me)
}

func (rf *Raft) CompressLogHandler(index int) {
	//处理kvservice 发来的压缩日志请求

	// log := rf.logs[1 : index-rf.lastIncludedIndex+1]

	// // data := rf.persister.EncodingLog(log)

	// rf.Snapshot(index, data)

}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int32
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int32
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Team        int32
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).

	// rf.mu.Lock()
	judge := rf.voteTerm < args.Term
	// fmt.Printf("index %v 的rf.voteTerm %v: args index %v,term %v,judge :%v\n",
	// 	rf.me, rf.voteTerm, args.CandidateId, args.Term, judge)
	// rf.mu.Unlock()

	judge2 := (args.Term >= rf.currentTerm || rf.votedFor == int32(args.CandidateId))
	judge3 := int32(args.LastLogTerm) >= rf.logLastTerm
	// fmt.Printf("&&&&&args  lastterm %v rf index %v rflogTerm %v\n",
	// 	args.LastLogTerm, rf.me, rf.logLastTerm)

	reply.Team = rf.currentTerm
	// fmt.Printf("index %v j1 %v j2 %v j3 %v\n", rf.me, judge, judge2, judge3)
	if judge && judge2 && judge3 {
		if int(args.LastLogTerm) == int(rf.logLastTerm) && args.LastLogIndex < int(rf.logLastIndex) {
			reply.VoteGranted = false
			return
		}
		//同意投票
		rf.state = "F"
		reply.VoteGranted = true
		rf.votedFor = int32(args.CandidateId)
		rf.currentTerm = args.Term
		rf.isLogCommon = false
		rf.heartbeat.now++
		atomic.AddInt32(&rf.voteTerm, 1)
		// fmt.Printf("index%v 在任期 %vterm 投票给 index %v \n",
		// 	rf.me, rf.currentTerm, args.CandidateId)
		rf.persist()
		return
	}
	reply.VoteGranted = false
	// fmt.Printf("index %v拒接投票%v\n", rf.me, args.CandidateId)
	// fmt.Printf("投过票了 index me %v  for %v 任期 %v\n",
	// 	rf.me, args.CandidateId, rf.currentTerm)
	return
}

func (rf *Raft) RequestVoteRunTime(k int,
	args *RequestVoteArgs, reply *RequestVoteReply) {
	// fmt.Printf("idnex %v 向  %v 请求投票 状态%v loglastIndex %v \n",
	// 	rf.me, k, rf.state, rf.logLastIndex)
	b := rf.sendRequestVote(k, args, reply)
	if !b || !reply.VoteGranted {
		// fmt.Printf("C %v 奇怪的出错 index %v\n", rf.me, k)
		return
	}
	rf.voteNum++
	// rf.voteDoneNum++
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	reply.Term = rf.currentTerm
	//定位appwei
	// fmt.Printf("最初的index %v len %v  log %p\n", rf.me, len(rf.logs), &(rf.logs))
	if args.Entries == nil && args.LLeft == -1 {

		//表示心跳
		if args.Term >= rf.currentTerm && args.LeaderId == int(rf.votedFor) {
			rf.state = "F"
			rf.heartbeat.now++
			rf.currentTerm = args.Term
			rf.votedFor = int32(args.LeaderId)

			reply.Success = true
			// fmt.Printf("L index %v给 index %v 心跳 term %v\n",
			// 	args.LeaderId, rf.me, rf.currentTerm)
			// && rf.commitDone
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
	} else {

		if args.Term < rf.currentTerm || rf.votedFor != int32(args.LeaderId) {
			// fmt.Printf("奇怪了奇怪了 rf.me %v Term %v votefor %v\n",
			// 	rf.me, rf.currentTerm, rf.votedFor)
			reply.Success = false
			reply.Term = rf.currentTerm
			reply.LRight = -1
			return
		}

		// fmt.Printf("&*&** argsTerm %v rf T %v\n", args.Term, rf.currentTerm)
		reply.Term = rf.currentTerm
		// fmt.Printf("index %v len %v  log %p\n", rf.me, rf.logLastIndex, &(rf.logs))

		//日志最后的索引
		index := rf.logLastIndex
		if rf.lastIncludedIndex > 0 {
			//已经有快照，日志索引需要重新映射
			index = rf.logLastIndex - rf.lastIncludedIndex
		}

		if len(rf.logs) == 1 &&
			(args.PrevLogIndex <= 0 || args.PrevLogIndex == rf.lastIncludedIndex) {
			//初始日志为空，不需要检查一致性
			// fmt.Printf("开始开始 nextIndex %v term %v\n",
			// 	args.PrevLogIndex, args.Term)
			rf.logs = append(rf.logs, args.Entries...)
			//保证数组索引和log Index 的一致性
			// rf.logLastIndex = len(rf.logs) - 1
			rf.logLastIndex = rf.lastIncludedIndex + len(rf.logs) - 1
			index = len(rf.logs) - 1
			rf.logLastTerm = rf.logs[index].Term
			rf.currentTerm = rf.logs[index].Term
			reply.Success = true
			rf.isLogCommon = true
			rf.persist()
			return
		}

		// fmt.Printf("index %v 需要检查Lindex %v prenext %v argsloglen %v\n",
		// 	rf.me, args.LeaderId, args.PrevLogIndex, len(args.Entries))
		// for _, v := range rf.logs {
		// 	fmt.Printf("Value: %#v \n", v)
		// }

		//检查一致性
		lastIndex := rf.logLastIndex - rf.lastIncludedIndex
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

		} else {
			if args.First {
				//第一次，返回
				reply.LLeft = args.LLeft
				reply.LRight = args.LRight
				reply.IsOk = false
				// fmt.Println("第一次放回")
				return
			}
			//需要与主节点达成一致，然后开始复制日志
			if args.PrevLogIndex > rf.logLastIndex {
				//长度不符合 ,
				reply.LLeft = args.LLeft
				reply.LRight = args.PrevLogIndex - 1
				// fmt.Printf("哈哈哈哈 l%v r%v 地址 %v\n",
				// 	reply.LLeft, reply.LRight, &reply)
				if reply.LRight == 0 {
					reply.LRight = -100
				}

				reply.IsOk = false
				return
			}

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
				// fmt.Printf("2添加成功 index %v lastTerm %v\n", rf.me, rf.logLastTerm)
				rf.persist()
				return

			}

			if args.PrevLogIndex-rf.lastIncludedIndex <= 0 || rf.logs[args.PrevLogIndex-rf.lastIncludedIndex].Term == args.PrevLogTerm {
				//通过，继续尝试寻找
				reply.IsOk = true
				reply.LLeft = args.PrevLogIndex + 1
				reply.LRight = args.LRight
				// fmt.Printf("+++rf %v 匹配成功 mid %v l %v r %v\n",
				// 	rf.me, args.PrevLogIndex, reply.LLeft, reply.LRight)
				return
			} else {
				//不通过，
				reply.IsOk = false
				reply.LLeft = args.LLeft
				reply.LRight = args.PrevLogIndex - 1
				// fmt.Printf("不通过，哈哈 l %v  r%v\n", reply.LLeft, reply.LRight)
				return
			}

		}

		// for i := len(rf.logs) - 1; i > 0; i-- {
		// 	fmt.Printf("比较 rf.me %v log[%v].Term is %v \n",
		// 		rf.me, i, rf.logs[i].Term)
		// 	if rf.logs[i].Term == args.PrevLogTerm && i == args.PrevLogIndex {
		// 		//一致性通过，添加日志
		// 		if args.Adjust {
		// 			fmt.Printf("index %v 一致的地方是 %v\n", rf.me, i)
		// 			reply.Success = true
		// 			return
		// 		}

		// 		if i == len(rf.logs)-1 {
		// 			rf.logs = append(rf.logs, args.Entries...)
		// 			rf.logLastIndex = len(rf.logs) - 1
		// 			rf.currentTerm = rf.logs[rf.logLastIndex].Term
		// 			rf.logLastTerm = rf.logs[rf.logLastIndex].Term
		// 			reply.Success = true
		// 			rf.isLogCommon = true
		// 			fmt.Printf("1添加成功 index %v lastTerm %v loglastindex %v\n", rf.me, rf.logLastTerm, rf.logLastIndex)
		// 			rf.persist()
		// 			return
		// 		} else {
		// 			//需要进行切片
		// 			rf.logs = rf.logs[:i+1]
		// 			rf.logs = append(rf.logs, args.Entries...)
		// 			rf.logLastIndex = len(rf.logs) - 1
		// 			rf.currentTerm = rf.logs[rf.logLastIndex].Term
		// 			rf.logLastTerm = rf.logs[rf.logLastIndex].Term
		// 			reply.Success = true
		// 			rf.isLogCommon = true
		// 			rf.persist()
		// 			fmt.Printf("2添加成功 index %v lastTerm %v\n", rf.me, rf.logLastTerm)
		// 			rf.persist()

		// 			return
		// 		}
		// 	}
		// }
	}
}

func (rf *Raft) AppendEntriesRunTime(k int, Done *int32,
	args *AppendEntriesArgs, reply *AppendEntriesReply) {

	//定位2

	// fmt.Printf("rf %v 向 %v发的日志长度 %v  ,\n", rf.me, k, len(args.Entries))
	b := rf.sendAppendEntries(k, args, reply)
	if len(args.Entries) != 0 {
		// fmt.Printf("Lindex %v向index%v 发布日志内容  log %#v argsIndex %v argsTerm  %v\n",
		// 	rf.me, k, args.Entries[0], args.PrevLogIndex, args.Term)
	}
	if b == false || reply.LRight == -1 {
		//节点失去连接
		// fmt.Printf("rm %v 添加出错了%v\n", k, reply.Success)
		rf.mu.Lock()
		rf.isAppendDone[k] = true
		rf.mu.Unlock()
		return
	}

	if reply.Term > rf.currentTerm {
		rf.state = "F"
		rf.currentTerm = reply.Term
		rf.isLogCommon = false
		rf.KillLCommitRunTime()
		rf.persist()
		rf.isAppendDone[k] = true
		return
	}
	//开始比较

	var index int
	for i := 0; reply.Success == false; i++ {

		// fmt.Printf("@@@@@ k is %v i is %v l %v r %v 地址 %v\n",
		// 	k, i, reply.LLeft, reply.LRight, &reply)

		if reply.LRight == -100 {
			//用来改写因为rpc出现默认值重写旧值
			//gob编码会忽略零值字段，除非它们是数组的元素。
			//这意味着如果reply的LRight字段是0，那么它在编码的时候，
			// 就不会被写入到字节流中。而在解码的时候，如果没有读到这个字段，
			// 它就会把reply的LRight字段设置为默认值，也就是为什么之前发送RPC时的初值。
			// 这就解释了为什么reply的LRight字段会被覆写
			reply.LRight = 0
		}

		// if reply.IsOk == true {
		// 	rf.nextIndex[k] = 1
		// 	args.Entries = nil
		// }

		//调整nextIndex
		// rf.nextIndex[k]--
		// i := rf.nextIndex[k]
		// args.PrevLogIndex--
		// if i-1 < 0 {
		// 	rf.nextIndex[k] = 1
		// 	return
		// }
		// args.PrevLogTerm = rf.logs[i-1].Term
		// // args.Entries = rf.logs[i:]

		//二修改
		// if reply.FMidIndex > rf.logLastIndex {
		// 	args.IsOk = false
		// 	args.Entries = nil
		// 	args.FL = reply.FL
		// 	args.FR = reply.FR
		// 	rf.sendAppendEntries(k, &args, &reply)
		// 	continue
		// } else {

		// 	if rf.logs[reply.FMidIndex].Term == reply.FMidTerm {

		// 		//通过，尝试继续查找
		// 		args.FL = reply.FMidIndex + 1
		// 		args.FR = reply.FR
		// 		if args.FL > args.FR {
		// 			//最佳匹配，开始发送日志
		// 			index := rf.logLastIndex
		// 			args.Entries = rf.logs[reply.FMidIndex+1 : index]
		// 			if index == reply.FMidIndex+1 {
		// 				args.Entries = rf.logs[reply.FMidIndex:index]
		// 			}
		// 			args.IsOk = true
		// 		}

		// 	}
		// }

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

			// fmt.Printf("检查 l %v r %v mid %v last %v\n",
			// 	args.LLeft, args.LRight, mid, rf.lastIncludedIndex)
			args.PrevLogTerm = rf.logs[index].Term
			args.Entries = nil
			args.First = false

			//开始发送
			// fmt.Printf("二分匹配 rm %vargs LL %v LR %v mid %v\n",
			// 	k, args.LLeft, args.LRight, args.PrevLogIndex)
			rf.sendAppendEntries(k, args, reply)

		} else {
			if reply.LRight < 0 {
				//寄了，匹配不到
				// fmt.Println("寄了寄了寄了*********")
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
			// if index+1 == bestIndex+1 {
			// 	args.Entries = rf.logs[bestIndex:]
			// }
			args.PrevLogIndex = reply.LRight
			args.PrevLogTerm = rf.logs[bestIndex].Term
			args.LLeft = reply.LLeft
			args.LRight = reply.LRight
			rf.nextIndex[k] = reply.LRight + 1
			//开始发送k
			// fmt.Printf("最佳二分匹配 rm %vargs LL %v LR %v mid %v\n",
			// 	k, args.LLeft, args.LRight, bestIndex)
			rf.sendAppendEntries(k, args, reply)

		}

	}
	// temp := rf.nextIndex[k]
	rf.nextIndex[k] += len(args.Entries)
	// fmt.Printf("index %v 的nextindex %v -> %v last %v \n ", k, temp, rf.nextIndex[k], rf.logLastIndex)
	rf.matchIndex[k] = rf.nextIndex[k] - 1
	//k节点同步成功，发送管道信息
	// fmt.Printf("^^^^管道发送 appendIndex %v\n", rf.matchIndex[k])
	rf.lCommitChan <- AppendDone{
		k:           k,
		AppendIndex: rf.matchIndex[k],
		IsExit:      false,
	}
	rf.mu.Lock()
	rf.isAppendDone[k] = true
	rf.mu.Unlock()
}

func (rf *Raft) SendAppendLogsHandler() {

	for true {

		//轮询时间
		time.Sleep(17 * time.Millisecond)

		if rf.state != "L" {
			return
		}
		Term := rf.currentTerm
		lastIndex := rf.logLastIndex
		var Done int32
		Done = 1

		// fmt.Printf("^^^^^^^^^^^L %v statate %v lastIndex is %v 时间 地址……%v \n",
		// 	rf.me, rf.state, lastIndex, &rf)
		for k := 0; k < len(rf.peers); k++ {
			if rf.state != "L" {
				return
			}
			if k == rf.me {
				continue
			}
			// fmt.Printf("^^^^^^^^^^^rf.index %v 节点 k %v lastIndex is %v \n",
			// 	rf.me, k, rf.matchIndex[k])
			if rf.matchIndex[k] != lastIndex {
				// logs := []*Log{}
				// logs = append(logs, rf.logs[rf.preAppendLogIndex:]...)
				// fmt.Printf("index %v 开始发送日志 %v term %v\n", rf.me, command, rf.currentTerm)
				//开始复制日志
				if rf.state != "L" {
					return
				}
				if k == rf.me {
					continue
				}

				rf.mu.Lock()
				ok := rf.isAppendDone[k]
				rf.mu.Unlock()

				if !ok {
					// fmt.Printf("rf %v 未完成任务\n", k)
					continue
				}

				if rf.nextIndex[k] <= rf.lastIncludedIndex {
					//发送快照
					rf.isAppendDone[k] = false
					rf.lSnapshotChan <- LSnapshot{
						k: k,
					}
					continue
				}
				// if !rf.isAppendDone[k] {
				// 	continue
				// }

				i := rf.nextIndex[k] - 1 - rf.lastIncludedIndex
				if i <= 0 {
					time.Sleep(70 * time.Millisecond)
				}
				var args AppendEntriesArgs
				nextIndex := rf.nextIndex[k]
				// if nextIndex == lastIndex+1 {
				// 	nextIndex--
				// }

				index := nextIndex - rf.lastIncludedIndex
				if index < 0 {
					continue
				}
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

			}

		}
		// ms1 := 200
		// time.Sleep(time.Duration(ms1) * time.Millisecond)

		// if int(Done) >= len(rf.peers)/2+1 && lastIndex > rf.commitIndex {

		// 	for i := rf.preCommitIndex + 1; i <= lastIndex; i++ {
		// 		x := ApplyMsg{
		// 			CommandValid: true,
		// 			Command:      rf.logs[i].Command,
		// 			CommandIndex: i,
		// 		}
		// 		rf.applyCh <- x
		// 		fmt.Printf("*********{lindex %v commitindex %v log %v pre %v i %v \n",
		// 			rf.me, lastIndex, rf.logs[i].Command, rf.preCommitIndex, i)
		// 	}
		// 	rf.preCommitIndex = lastIndex
		// 	rf.commitIndex = lastIndex
		// }
		// rf.preAppendLogIndex = lastIndex
		// rf.persist()

	}

}

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

func (rf *Raft) InstallSnapshotRunTime() {
	//用来接收向F 发送日志快照的请求
	for true {

		select {
		case v, _ := <-rf.lSnapshotChan:
			if v.IsExit == true {
				return
			}
			// fmt.Printf("执行了*********** rm %v \n", v.k)
			//向k 发送快照
			args := InstallSnapshotArgs{
				Term:              rf.currentTerm,
				LeaderId:          rf.me,
				LastIncludedIndex: rf.lastIncludedIndex,
				LastIncludedTerm:  rf.lastIncludedTerm,
				Data:              rf.persister.ReadSnapshot(),
			}
			reply := InstallSnapshotReply{}

			res := rf.sendInstallSnapshot(v.k, &args, &reply)
			if res {

				rf.matchIndex[v.k] = args.LastIncludedIndex
				rf.nextIndex[v.k] = rf.matchIndex[v.k] + 1
				// rf.isAppendDone[v.k] = true
				// fmt.Printf("Done 完成 %v\n", v.k)
			}
			rf.isAppendDone[v.k] = true

		}

	}

}

func (rf *Raft) KillLCommitRunTime() {

	// select 	{
	// case rf.lCommitChan <- :
	// 	// 数据成功写入管道
	// case <-closedCh:
	// 	// 管道已关闭的处理逻辑
	// default:
	// 	// 管道已满，无法写入数据
	// }

	appendDone := AppendDone{
		IsExit: true,
	}

	select {
	case rf.lCommitChan <- appendDone:
		// 写入通道成功
	case <-time.After(100 * time.Millisecond):
		// 在一秒内无法写入通道
		// 可以视为通道已关闭
		// fmt.Println("————————管道已经关闭")
		return
	}

	// rf.lCommitChan <- AppendDone{
	// 	IsExit: true,
	// }

	// rf.lCommitChan <- appendDone
}

func (rf *Raft) LCommitLogRunTime() {

	// fmt.Printf("index %v 主节点监听进程启动", rf.me)
	commitMap := make(map[int]int, 0)

	for true {

		select {
		case v, ok := <-rf.lCommitChan:

			// fmt.Printf("F %v d commitIndex %v\n", rf.me, rf.commitIndex)

			// start := time.Now()

			if ok && !v.IsExit {

				if v.AppendIndex <= rf.commitIndex {
					//已经提交了，不做处理
					// fmt.Println("已经提交，******")
					continue
				} else {
					// commitIndex := rf.commitIndex - rf.lastIncludedIndex
					// fmt.Printf("VappendIndex %v loglast %v", v.AppendIndex, rf.logLastIndex)
					commitMap[v.AppendIndex]++
					index := commitMap[v.AppendIndex]
					// fmt.Printf("打算index 是 这个 %v------- rm index %v commitIndex %v appIndex %v\n",
					// 	index, rf.me, rf.commitIndex, v.AppendIndex)
					if index >= len(rf.peers)/2 {
						//到达提交条件,开始提交
						for i := rf.commitIndex + 1; i <= v.AppendIndex; i++ {

							logIndex := i - rf.lastIncludedIndex
							commad := rf.logs[logIndex].Command

							x := ApplyMsg{
								CommandValid: true,
								Command:      commad,
								CommandIndex: i,
							}

							if i == v.AppendIndex {
								x.IsLast = true
							}

							// fmt.Printf("开始发送到应用机 me %v , i %v,  append %v \n", rf.me, i, v.AppendIndex)
							rf.applyCh <- x

							// fmt.Printf("*********{lindex %v commitindex %v commad %v  pre %v i %v AppendIndex %v\n",
							// 	rf.me, index, commad, rf.preCommitIndex, i, v.AppendIndex)

						}
						rf.commitIndex = v.AppendIndex
						rf.persist()
						delete(commitMap, v.AppendIndex)
					}
					// i, i2, i3 := rf.GetNowTime()
					// fmt.Printf("*******主提交结束当前时间是 %02d 分 %02d 秒 %d 毫秒\n",
					// 	i, i2, i3)
				}

				// dur := time.Since(start)
				// fmt.Printf("!!!!!! 主节点发送花费的时间为 %v\n", dur)

			} else {
				return
			}

		}

	}

}
func (rf *Raft) FCommitLogRunTime() {

	for true {

		select {
		case v, ok := <-rf.fCommitChan:

			// fmt.Printf("F %v d commitIndex %v\n", rf.me, rf.commitIndex)
			if ok {
				//准备提交
				if v.PrevLogIndex-rf.lastIncludedIndex < 0 {
					// fmt.Printf("索引超出，数组溢出*********************")
					continue
				}

				if v.PrevLogIndex == 0 ||
					v.PrevLogIndex > rf.logLastIndex ||
					rf.logs[v.PrevLogIndex-rf.lastIncludedIndex].Term != v.PrevLogTerm {
					// fmt.Printf("*^^^^^^^ rf index %v rf Term %v args.Index %v args.Term %v len %v\n",
					// 	v.PrevLogIndex-rf.lastIncludedIndex, rf.logs[v.PrevLogIndex-rf.lastIncludedIndex].Term,
					// 	v.PrevLogIndex, v.PrevLogTerm, len(rf.logs))

					continue
				}

				// fmt.Printf("@@@@@@@@@@@v pre index %v term %v\n", v.PrevLogIndex, v.PrevLogTerm)
				min := math.Min(float64(rf.logLastIndex), float64(v.CommitIndex))
				lastCommitIndex := rf.commitIndex + 1
				// rf.commitIndex = int(min)
				// fmt.Printf("准备了 index %v, min %v loglen %v commitIndex %v\n",
				// 	rf.me, min, rf.logLastIndex, v.CommitIndex)
				// fmt.Printf("子节点 lastcomm %v min %v\n", lastCommitIndex, min)
				for i := lastCommitIndex; i <= int(min); i++ {
					index := i - rf.lastIncludedIndex
					if index < 0 {
						// fmt.Printf("<>>><><><>><><><><<> i %v lastIncl %v index %v\n",
						// 	rf.me, rf.lastIncludedIndex, index)
						return
					}
					commad := rf.logs[index].Command
					// rf.logs[index].Command
					msg := ApplyMsg{
						CommandValid: true,
						Command:      commad,
						CommandIndex: i,
					}
					if i == int(min) {
						msg.IsLast = true
					}
					rf.applyCh <- msg
					rf.commitIndex = i
					// select {
					// case rf.applyCh <- msg:
					// 	// 写入成功
					// default:
					// 	fmt.Printf("index %v 管道写入失败了>>>>>>>>>>>\n", rf.me)
					// }
					// fmt.Printf("----------index %v i %v 在 commitindex %v commad %v  loglast %v Term %v \n",
					// 	rf.me, i, rf.commitIndex, commad, rf.logLastIndex, rf.logLastTerm)
					// i, i2, i3 := rf.GetNowTime()
					// // fmt.Printf("++++++++节点开始当前时间是 %02d 分 %02d 秒 %d 毫秒\n",
					// // 	i, i2, i3)
				}

			}

		}
	}
}

func (rf *Raft) CommitLogRunTime(args AppendEntriesArgs) {
	rf.mu.Lock()
	if !rf.isLogCommon {
		rf.mu.Unlock()
		return
	}
	// fmt.Printf("index %v 开始提交\n", rf.me)
	rf.commitDone = false
	if args.LeaderCommit > rf.commitIndex {
		min := math.Min(float64(rf.logLastIndex), float64(args.LeaderCommit))
		// max := math.Max(float64(rf.logLastIndex), float64(args.LeaderCommit))
		lastCommitIndex := rf.commitIndex + 1
		rf.commitIndex = int(min)
		// fmt.Printf("准备了 index %v,lastCommit %v min %v loglen %v\n",
		// 	rf.me, lastCommitIndex, min, rf.logLastIndex)
		for i := lastCommitIndex; i <= int(min); i++ {

			//防止节点还未同步，导致数组索引溢出
			if i > rf.logLastIndex {
				time.Sleep(time.Millisecond * 100)
				// fmt.Printf("这个测试的log len %v lastlog %v\n", len(rf.logs)-1, rf.logLastIndex)
			}

			msg := ApplyMsg{
				CommandValid: true,
				Command:      rf.logs[i].Command,
				CommandIndex: i,
			}

			rf.applyCh <- msg
			// fmt.Printf("----------index %v 在 commitindex %v log %v loglast %v\n",
			// 	rf.me, rf.commitIndex, rf.logs[i].Command, rf.logLastIndex)
		}
		rf.preCommitIndex = rf.commitIndex
		// fmt.Printf("index %v 提交 commitindex %v \n", rf.me, rf.commitIndex)
	}
	rf.commitDone = true
	rf.mu.Unlock()
	rf.persist()
	return
}

func (rf *Raft) SendCommitIndex(commitIndex int) {

	args := AppendEntriesArgs{
		Term:         rf.currentTerm,
		Entries:      nil,
		LeaderId:     rf.me,
		LeaderCommit: commitIndex,
	}

	for k, _ := range rf.peers {
		//给所有节点发送心跳请求
		if k == rf.me {
			continue
		}
		reply := AppendEntriesReply{}

		if rf.state != "L" {
			// fmt.Printf("index %v已经不是领导者了 %v\n", rf.me, rf.state)
			return
		}

		// time.Sleep(time.Duration(60) * time.Millisecond)
		go rf.HeartbeatRunTime(k, args, reply)
	}

}

func (rf *Raft) HeartbeatRunTime(k int,
	args AppendEntriesArgs, reply AppendEntriesReply) {

	rf.sendAppendEntries(k, &args, &reply)
	if reply.Term > rf.currentTerm {
		// fmt.Printf("rf.me %v 被 %v 改变了!!!!", rf.me, k)
		rf.state = "F"
		rf.currentTerm = reply.Term
		rf.isLogCommon = false
		rf.KillLCommitRunTime()
		rf.persist()
		time.Sleep(50 * time.Millisecond)
		return
	}
}

func (rf *Raft) sendHeartbeat() {

	for true {

		for k, _ := range rf.peers {
			//给所有节点发送心跳请求
			if k == rf.me {
				continue
			}
			index := rf.matchIndex[k] - rf.lastIncludedIndex
			preIndex := rf.matchIndex[k]
			var Term int32
			if index >= 0 {
				Term = rf.logs[rf.matchIndex[k]-rf.lastIncludedIndex].Term
				if index == 0 {
					Term = rf.lastIncludedTerm
				}
			} else {
				//用户还没有匹配
				preIndex = 0
				Term = rf.lastIncludedTerm
				// fmt.Printf("Term ********** %v index %v\n", Term, k)
			}

			args := AppendEntriesArgs{
				Term:         rf.currentTerm,
				Entries:      nil,
				LeaderId:     rf.me,
				LeaderCommit: rf.commitIndex,
				PrevLogIndex: preIndex,
				PrevLogTerm:  Term,
				LLeft:        -1,
				// PrevLogIndex: rf.nextIndex[k] - 1,
				// PrevLogTerm:  rf.logs[rf.nextIndex[k]-1].Term,
			}
			reply := AppendEntriesReply{}

			if rf.state != "L" {
				// fmt.Printf("index %v已经不是领导者了 %v\n", rf.me, rf.state)
				return
			}
			time.Sleep(time.Duration(50) * time.Millisecond)
			go rf.HeartbeatRunTime(k, args, reply)
			// rf.sendAppendEntries(k, &args, &reply)
			// if reply.Term > rf.currentTerm {
			// 	rf.state = "F"
			// 	rf.currentTerm = reply.Term
			// 	return
			// }
		}
	}

}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {

	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}
func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {

	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// time.Sleep(100 * time.Millisecond)

	// i, i2, i3 := rf.GetNowTime()
	// fmt.Printf("*******开始当前时间是 %02d 分 %02d 秒 %d 毫秒\n",
	// 	i, i2, i3)

	index := -1
	term := -1
	isLeader := false
	nowTerm := rf.currentTerm
	// logs := []*Log{}
	log := Log{
		Command: command,
		Term:    rf.currentTerm,
	}
	// var Done int32

	// Done = 1
	if rf.state != "L" {
		return index, term, isLeader
	}

	// fmt.Printf("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<\n")
	// fmt.Printf("index %v state %v\n", rf.me, rf.state)
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

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.\
	rf.persist()
	// fmt.Printf("index %v 下线 ：cuuentterm %v 身份%v log len %v \n",
	// 	rf.me, rf.currentTerm, rf.state, rf.logLastIndex)
	// fmt.Printf("index %v Value: ", rf.me)
	// for _, v := range rf.logs {
	// 	fmt.Printf(" %#v ", v.Command)
	// }
	if rf.state == "L" {
		rf.KillLCommitRunTime()
	}
	// defer close(rf.fCommitChan)
	// defer close(rf.lCommitChan)

}

//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.
		//设置选举超时时间
		ms1 := 300 + (rand.Int63() % 400)
		time.Sleep(time.Duration(ms1) * time.Millisecond)

		if rf.heartbeat.IsNormal(rf.me) || rf.state == "L" {
			//获得心跳请求正常
			// fmt.Printf("index %rv 调用normal \n", rf.me)
			continue
		} else {
			//心跳请求异常,开始选举

			rf.state = "C"
			rf.currentTerm = atomic.AddInt32(&Term, 1)
			// fmt.Printf("index %v 开始选举 term %v lastlogTerm %v\n", rf.me, rf.currentTerm, rf.logLastTerm)
			if rf.currentTerm <= rf.voteTerm {
				// fmt.Printf("index %v在 %v term已经投过票了\n ", rf.me, rf.currentTerm)
				continue
			}
			rf.voteNum = 1
			atomic.AddInt32(&rf.voteTerm, 1)

			args := RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  rf.me,
				LastLogIndex: rf.logLastIndex,
				LastLogTerm:  rf.logLastTerm,
			}
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
				// fmt.Printf("idnex %v 向  %v 请求投票 \n", rf.me, k)
				// b := rf.sendRequestVote(k, &args, &reply)
				// if !b {
				// 	fmt.Printf("C %v 奇怪的出错 index %v\n", rf.me, k)
				// }
				// if reply.VoteGranted {
				// 	//统计票数
				// 	rf.voteNum++
				// }

			}
			// fmt.Printf("index %v 任期 %v 票数为 %v\n", rf.me, rf.currentTerm, rf.voteNum)

			// if rf.state != "C" {
			// 	continue
			// }
			//节点超时时间
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

			// fmt.Printf("index %v 选举失败\n", rf.me)

			rf.persist()

		}

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		ms := 50 + (rand.Int63() % 300)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		// fmt.Printf("另外的回合 %v\n", ms)
	}
}

func (rf *Raft) LeaderReset() {
	i := rf.logLastIndex
	for k, _ := range rf.peers {
		rf.nextIndex[k] = i + 1

	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.votedFor = -1
	rf.state = "F"
	rf.applyCh = applyCh
	rf.logs = make([]*Log, 1)
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	rf.commitDone = true
	rf.lCommitChan = make(chan AppendDone)
	rf.fCommitChan = make(chan FCommit)
	rf.lSnapshotChan = make(chan LSnapshot)
	rf.isAppendDone = map[int]bool{}

	for k, _ := range rf.peers {
		rf.nextIndex[k] = 1
		rf.isAppendDone[k] = true
	}
	rf.logs[0] = &Log{
		Term: -1,
	}
	// fmt.Printf("index %v len %v 上线了\n", rf.me, len(rf.logs))

	// Your initialization code here (2A, 2B, 2C).

	// fmt.Printf("&&&*****&&&&& rf %v 已经被创建 地址 %v\n", rf.me, &rf)
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	if rf.state == "L" {
		go rf.sendHeartbeat()
		go rf.SendAppendLogsHandler()
		go rf.LCommitLogRunTime()
		go rf.InstallSnapshotRunTime()
	}
	go rf.FCommitLogRunTime()
	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
