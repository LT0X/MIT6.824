package kvraft

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"6.5840/labrpc"
)

//表示分配给op的唯一Id
var opId int32

type Clerk struct {
	servers []*labrpc.ClientEnd

	channelPool *ChannelPool //客户端使用的管道池

	leaderIndex int //表示Service leader的索引缓存

	// serverLastOpId int
	// You will have to modify this struct.
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	ck.leaderIndex = int(nrand()) % len(servers)
	ck.channelPool = NewChannelPool(4)

	//等待raft初步启动
	time.Sleep(300 * time.Millisecond)
	// You'll have to add code here.
	return ck
}

func (ck *Clerk) monitorRPCRunTime(channel chan bool,
	gReply *GetReply, pReply *PutAppendReply) {

	time.Sleep(time.Millisecond * 800)

	if gReply != nil {

		if !gReply.Done {

			//当前缓存server不能正常处理
			gReply.NotLeader = true
			channel <- true
		}
		return
	}

	if pReply != nil {

		if !pReply.Done {
			//当前缓存server不能正常处理
			pReply.NotLeader = true
			channel <- true
		}
		return
	}

}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.

	args := GetArgs{

		Key: key,
		Id:  int(atomic.AddInt32(&opId, 1)),
	}
	reply := GetReply{
		NotLeader: false,
	}
	fmt.Printf("客户端开始发送  opid %v op %v \n", args.Id, args)

	// channel := ck.channelPool.GetChannel(int32(args.Id))

	// go ck.monitorRPCRunTime(channel, &reply, nil)
	// start := time.Now()
	for !reply.Done {

		if reply.NotLeader {
			ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)

		}

		// ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)
		// fmt.Printf("发送 kv 的缓存 %v\n", ck.leaderIndex)
		ok := ck.sendGetRPC(ck.leaderIndex, &args, &reply)

		if !ok {
			fmt.Printf("客户端发起RPC失败 opid %v,reply %v,cklindex %v\n",
				args.Id, reply, ck.leaderIndex)

			reply.NotLeader = true
			// if ck.serverLastOpId >= args.Id {
			// 	reply.Done = true
			// 	fmt.Printf("客户端退出了 opid %v\n", args.Id)
			// }
		}

	}

	// for !reply.Done {

	// 	// if reply.NotLeader {
	// 	// 	ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)
	// 	// }

	// 	go ck.sendGetRPCRunTime(&args, &reply, channel)

	// 	select {
	// 	case v, _ := <-channel:
	// 		if v {
	// 			continue
	// 		}
	// 	}

	// }
	// dur := time.Since(start)
	// fmt.Printf("!!!!!! Get花费的时间为 %v\n", dur)

	fmt.Printf("opid %v get 完成了 \n", args.Id)
	return reply.Value
}

func (ck *Clerk) sendGetRPC(index int, args *GetArgs, reply *GetReply) bool {

	return ck.servers[index].Call("KVServer.Get", args, reply)
}

func (ck *Clerk) sendGetRPCRunTime(
	args *GetArgs, reply *GetReply, channel chan bool) {

	ok := false
	for !ok {

		if reply.NotLeader {
			ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)
		}

		ok = ck.sendGetRPC(ck.leaderIndex, args, reply)
		if !ok {
			fmt.Printf("客户端发起RPC失败 opid %v,reply %v\n", args.Id, reply)
			reply.NotLeader = true
			// if ck.serverLastOpId >= args.Id {
			// 	reply.Done = true
			// 	fmt.Printf("客户端退出了 opid %v\n", args.Id)
			// }
		}
	}
	fmt.Printf("opid %v 结束了 \n", args.Id)
	channel <- ok

}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
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
	fmt.Printf("客户端开始发送  opid %v args %v\n", args.Id, args)

	// channel := ck.channelPool.GetChannel(int32(args.Id))

	// go ck.monitorRPCRunTime(channel, nil, &reply)

	for !reply.Done {

		if reply.NotLeader {
			ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)

		}

		// ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)
		// fmt.Printf("发送 kv 的缓存 %v\n", ck.leaderIndex)
		ok := ck.sendPutAppendRPC(ck.leaderIndex, &args, &reply)

		if !ok {
			fmt.Printf("客户端发起RPC失败 opid %v,reply %v,cklindex %v\n",
				args.Id, reply, ck.leaderIndex)

			reply.NotLeader = true
			// if ck.serverLastOpId >= args.Id {
			// 	reply.Done = true
			// 	fmt.Printf("客户端退出了 opid %v\n", args.Id)
			// }
		}

	}

	fmt.Printf("opid %v put 完成了 \n", args.Id)

	// // start := time.Now()
	// for !reply.Done {

	// 	// if !reply.IsLeader {
	// 	// 	ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)
	// 	// }

	// 	go ck.sendPutAppendRPCRunTime(&args, &reply, channel)

	// 	select {
	// 	case v, _ := <-channel:
	// 		if v {
	// 			continue
	// 		}
	// 	}

	// }

	// dur := time.Since(start)
	// fmt.Printf("!!!!!! put花费的时间为 %v\n", dur)
	// fmt.Printf("||||||||||||||||")

}

func (ck *Clerk) sendPutAppendRPC(index int, args *PutAppendArgs, reply *PutAppendReply) bool {

	// fmt.Printf("准备发送索引是 %v\n", index)
	return ck.servers[index].Call("KVServer.PutAppend", args, reply)
}

func (ck *Clerk) sendPutAppendRPCRunTime(
	args *PutAppendArgs, reply *PutAppendReply, channel chan bool) {

	ok := false
	for !ok {

		if reply.NotLeader {
			ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)

		}

		// ck.leaderIndex = (ck.leaderIndex + 1) % len(ck.servers)
		// fmt.Printf("发送 kv 的缓存 %v\n", ck.leaderIndex)
		ok = ck.sendPutAppendRPC(ck.leaderIndex, args, reply)
		if !ok {
			fmt.Printf("客户端发起RPC失败 opid %v,reply %v,cklindex %v\n",
				args.Id, reply, ck.leaderIndex)

			reply.NotLeader = true

			// if ck.serverLastOpId >= args.Id {
			// 	reply.Done = true
			// 	fmt.Printf("客户端退出了 opid %v\n", args.Id)
			// }
		}
	}

	// fmt.Printf("opid 客户端结束 %v reply %v\n", args.Id, reply)
	fmt.Printf("opid %v 结束了 \n", args.Id)
	channel <- true

}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
