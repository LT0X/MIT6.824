package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.
type AssignedArgs struct {
}

type AssignedReply struct {
	NReduce int
	TaskNun int32  //表示分配的任务编号
	Task    string //表示分配的任务
	File    string //表示处理的输入文件
}

type MapperDoneArgs struct {
	File string
}

type MapperDoneReply struct {
	IsDone bool
}

type ReduceDoneArgs struct {
	TaskNum int32
}

type ReduceDoneReply struct {
	IsDone bool
}

type CoordinatorDoneArgs struct {
}

type CoordinatorDoneReply struct {
	IsDone bool
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
