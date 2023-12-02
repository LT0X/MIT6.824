package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string
	Id    int
	// "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Err            Err
	Done           bool
	NotLeader      bool
	ServerLastOpId int
}

type GetArgs struct {
	Key string
	Id  int
	// You'll have to add definitions here.
}

type GetReply struct {
	Err            Err
	Done           bool
	NotLeader      bool
	Value          string
	ServerLastOpId int
}
