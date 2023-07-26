package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for true {
		//分配任务
		reply := CallAssignedTasks()
		// fmt.Println(reply)
		if reply.Task == "m" {

			//分配到map任务
			kv := mapperHandle(reply.File, mapf)

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

			final := ReduceHandle(allKv, reducef)
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
	for res, _ := CallCoordinatorDone(); !res.IsDone; {
		//休眠两秒重新检测
		time.Sleep(time.Second * 2)
	}
	fmt.Println("worker 退出")
	return
}

func mapperHandle(filename string, mapf func(string, string) []KeyValue) *[]KeyValue {

	intermediate := []KeyValue{}

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()
	kva := mapf(filename, string(content))
	intermediate = append(intermediate, kva...)
	return &intermediate
}

func ReduceHandle(intermediate []KeyValue,
	reducef func(string, []string) string) *[]KeyValue {

	var final []KeyValue
	var i int
	sort.Sort(ByKey(intermediate))
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)
		intermediate[i].Value = output
		final = append(final, intermediate[i])

		i = j
	}

	return &final
}

func readIntermediateFile(fileName string) (*[]KeyValue, error) {

	var kva []KeyValue
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("打开文件出错:", err)
		return nil, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	for {
		var kv KeyValue
		if err := dec.Decode(&kv); err != nil {
			break
		}
		kva = append(kva, kv)
	}
	return &kva, nil

}

func createIntermediateFile(fileName string, intermediate []KeyValue) error {

	file, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Failed to create file:", err)
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)

	for _, kv := range intermediate {
		err = enc.Encode(&kv)
		if err != nil {
			fmt.Println("Failed to write file:", err)
			return err
		}
	}
	return nil

}

func createOutputFile(fileName string, final []KeyValue) {

	ofile, _ := os.Create(fileName)
	defer ofile.Close()
	for _, v := range final {
		fmt.Fprintf(ofile, "%v %v\n", v.Key, v.Value)
	}
}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

func CallAssignedTasks() AssignedReply {
	args := AssignedArgs{}
	reply := AssignedReply{}

	ok := call("Coordinator.AssignedTasks", &args, &reply)
	if !ok {
		fmt.Printf("call failed!\n")
	}
	return reply
}

func CallMapperDone(file string) (*MapperDoneReply, error) {

	args := MapperDoneArgs{
		File: file,
	}
	reply := MapperDoneReply{}

	ok := call("Coordinator.MapperDone", &args, &reply)
	if !ok {
		return nil, errors.New("call failed")
	}
	return &reply, nil

}

func CallReduceDone(taskNum int32) (*ReduceDoneReply, error) {

	args := ReduceDoneArgs{
		TaskNum: taskNum,
	}
	reply := ReduceDoneReply{}

	ok := call("Coordinator.ReduceDone", &args, &reply)
	if !ok {
		return nil, errors.New("call failed")
	}
	return &reply, nil
}

func CallCoordinatorDone() (*CoordinatorDoneReply, error) {

	args := CoordinatorDoneArgs{}
	reply := CoordinatorDoneReply{}

	ok := call("Coordinator.CoordinatorDone", &args, &reply)
	if !ok {
		return nil, errors.New("call failed")
	}
	return &reply, nil
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
