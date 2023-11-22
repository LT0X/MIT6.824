package kvraft

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raft"
)

const Debug = false

// var opIdMap map[int]bool //用于检查opid 是否已经被执行。

// var channelPool ChannelPool

// func init() {
// 	fmt.Printf("servicev 进行初始化操作\n")

// 	opIdMap = make(map[int]bool, 300)
// }

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	Optype string
	Key    string
	Value  string
	Id     int

	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type KVServer struct {
	mu          sync.Mutex
	me          int
	rf          *raft.Raft
	applyCh     chan raft.ApplyMsg
	kvMap       map[string]string //本地service键值对
	channelPool ChannelPool       //管道池
	opIdMap     map[int]bool
	clientMap   map[int]bool //表示客户端是否还和Server链接

	lastOpId int   //Cleark 最后的opId
	dead     int32 // set by Kill()

	rwLock sync.RWMutex

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
}

func (kv *KVServer) getMapValue(k string) (string, bool) {
	kv.rwLock.RLock()
	defer kv.rwLock.RUnlock()
	v, ok := kv.kvMap[k]

	return v, ok
}

func (kv *KVServer) putMapValue(k string, v string) {
	kv.rwLock.Lock()

	defer kv.rwLock.Unlock()

	kv.kvMap[k] = v

}

func (kv *KVServer) appendMapValue(k string, v string) {
	kv.rwLock.Lock()
	defer kv.rwLock.Unlock()

	kv.kvMap[k] = kv.kvMap[k] + v

}

func (kv *KVServer) getSyncMap(key int, maps map[int]bool) (bool, bool) {

	kv.rwLock.RLock()
	defer kv.rwLock.RUnlock()

	v, ok := maps[key]
	return v, ok

}

func (kv *KVServer) setSyncMap(key int, value bool, maps map[int]bool) {
	kv.rwLock.Lock()
	defer kv.rwLock.Unlock()

	maps[key] = value

}

func (kv *KVServer) applyChRunTime() {

	for true {
		select {

		case msg, _ := <-kv.applyCh:

			if op, ok := msg.Command.(Op); ok {
				fmt.Printf("测试《《《 : opid %v me %v op %v \n", op.Id, kv.me, op)

				if _, ok := kv.rf.GetState(); ok {
					c := kv.channelPool.GetChannel(int32(op.Id))
					_, ok := kv.rf.GetState()
					fmt.Printf("向管道发送结束请求 opID %v kvindex %v isLeader %v\n", op.Id, kv.me, ok)

					// kv.rwLock.RLock()

					// // v, ok := kv.clientMap[op.Id]

					// kv.rwLock.RUnlock()

					v, ok := kv.getSyncMap(op.Id, kv.clientMap)

					if v && ok {
						c <- op.Id
					}

					fmt.Printf("这个才是测试 me %v\n", kv.me)
				}

				// v, _ := kv.opIdMap[op.Id]

				if v, _ := kv.getSyncMap(op.Id, kv.opIdMap); v {
					fmt.Printf("结束了 *********** 错误 error opid %v me %v\n",
						op.Id, kv.me)
					continue
				}

				kv.executeOp(op)
				kv.setSyncMap(op.Id, true, kv.opIdMap)
				// kv.opIdMap[op.Id] = true

				fmt.Printf("结束了 ***********opid %v me %v\n", op.Id, kv.me)

			}

		}
	}
}

func (kv *KVServer) executeOp(op Op) {

	if op.Optype == "Put" {

		kv.putMapValue(op.Key, op.Value)

	} else if op.Optype == "Append" {

		if _, ok := kv.getMapValue(op.Key); !ok {

			kv.putMapValue(op.Key, op.Value)

			return
		}
		// kv.kvMap[op.Key] = kv.kvMap[op.Key] + op.Value
		kv.appendMapValue(op.Key, op.Value)

	}
}

func (kv *KVServer) monitorChannel(channel chan bool, opid int) {

	//对每次处理客户端请求进行监视
	//检查管道是否因为异常长时间不释放
	//如果800ms还未处理完成，则认为异常，强制结束请求

	time.Sleep(time.Millisecond * 3000)

	// v, ok := kv.opIdMap[opid]
	v, ok := kv.getSyncMap(opid, kv.opIdMap)
	if !ok || !v {
		fmt.Printf("监管已经去除 opid %v ok %v,v %v\n", opid, ok, v)
		channel <- false
	}

	// fmt.Printf("opid %v me %v 开始判断管道超时 v %v ok %v \n",
	// 	opid, kv.me, v, ok)

}

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
		reply.ServerLastOpId = kv.lastOpId
		fmt.Printf("已经处理，提前释放 opid %v \n", args.Id)
		if !IsExit {
			reply.Value = ""
			return
		}
		reply.Value = v
		return
	}

	_, _, isLeader = kv.rf.Start(op)

	// fmt.Printf("raft index %v me %v    ", x, kv.me)

	if !isLeader {
		if !isLeader {
			//不是leader节点，需要重新返回重试
			reply.NotLeader = true

			return
		}
	}

	//表示客户端正在于服务器链接

	// kv.rwLock.Lock()
	// kv.clientMap[args.Id] = true
	// kv.rwLock.Unlock()

	kv.setSyncMap(args.Id, true, kv.clientMap)

	defer func() {
		// kv.rwLock.Lock()
		// kv.clientMap[args.Id] = false
		// kv.rwLock.Unlock()
		kv.setSyncMap(args.Id, false, kv.clientMap)
	}()

	fmt.Printf("管道开始监听 opid %v me %v\n", args.Id, kv.me)
	c := kv.channelPool.GetChannel(int32(args.Id))
	// start := time.Now()

	// go kv.monitorChannel(c, args.Id)

	for true {

		select {
		case isok, _ := <-c:

			if isok == args.Id {

				// v, IsExit := kv.kvMap[args.Key]
				v, IsExit := kv.getMapValue(args.Key)
				reply.Done = true
				reply.ServerLastOpId = kv.lastOpId
				// fmt.Printf("管道准备释放 opid %v \n", args.Id)
				if !IsExit {
					reply.Value = ""

					return
				}

				reply.Value = v
				// dur := time.Since(start)
				// fmt.Printf("!!!!!! Get花费的时间为 %v\n", dur)
				return
			} else {
				continue
			}
			// else {
			// 	fmt.Printf("监管函数 强制下线 opid %v me %v\n", args.Id, kv.me)
			// 	reply.NotLeader = true
			// 	return
			// }

		case <-time.After(3 * time.Second):
			// 2s服务器未能处理，表示出现异常，返回false
			fmt.Printf("监管函数 强制下线 opid %v me %v\n", args.Id, kv.me)
			reply.NotLeader = true
			return
		}

	}

}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	// fmt.Printf("|||||||||||||||||||||||")

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

	// fmt.Printf("raft index %v me %v    ", x, kv.me)

	// fmt.Printf("<<<<< i %v term %v, isLeader %v\n", i, i2, isLeader)

	if !isLeader {
		//不是leader节点，需要重新返回重试
		// fmt.Printf("不是leader，开始 返回 %v\n", kv.me)
		reply.NotLeader = true

		return
	}

	//表示客户端正在于服务器链接
	// kv.rwLock.Lock()
	// kv.clientMap[args.Id] = true
	// kv.rwLock.Unlock()

	// defer func() {
	// 	kv.rwLock.Lock()
	// 	kv.clientMap[args.Id] = false
	// 	kv.rwLock.Unlock()
	// }()

	kv.setSyncMap(args.Id, true, kv.clientMap)

	defer func() {
		// kv.rwLock.Lock()
		// kv.clientMap[args.Id] = false
		// kv.rwLock.Unlock()
		kv.setSyncMap(args.Id, false, kv.clientMap)
	}()

	// fmt.Printf("管道开始监听 opid %v me %v \n", args.Id, kv.me)
	c := kv.channelPool.GetChannel(int32(args.Id))
	// start := time.Now()

	// go kv.monitorChannel(c, args.Id)

	for true {

		select {

		case isok, _ := <-c:
			// fmt.Printf("！！！！！！！！ isok me %v %v\n", isok, kv.me)

			if isok == args.Id {
				fmt.Printf("管道准备释放 opid %v me %v\n", args.Id, kv.me)
				reply.Done = true
				reply.ServerLastOpId = kv.lastOpId
				// dur := time.Since(start)
				// fmt.Printf("!!!!!! put花费的时间为 %v opid %v\n", dur, args.Id)
				return
			} else {
				continue
			}
			// else {

			// 	fmt.Printf("监管函数 强制下线 opid %v me %v\n", args.Id, kv.me)

			// 	return
			// }
		case <-time.After(3 * time.Second):
			fmt.Printf("监管函数 强制下线 opid %v me %v\n", args.Id, kv.me)
			reply.NotLeader = true
			return
		}

	}

	// kv.executeOp(*op)

	// select {

	// case msg := <-kv.applyCh:
	// 	if op, ok := msg.Command.(Op); ok {

	// 		fmt.Printf("测试《《《 : opid %v msg id %v \n", args.id, op.id)
	// 	}
	// }

}

// thetester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead ( without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate
	kv.kvMap = make(map[string]string)
	kv.channelPool = *NewChannelPool(80)

	kv.opIdMap = make(map[int]bool, 300)

	kv.clientMap = make(map[int]bool, 300)

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	go kv.applyChRunTime()
	// You may need initialization code here.

	return kv
}
