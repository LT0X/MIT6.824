
if rf.preAppendLogIndex < rf.logLastIndex {

	lastIndex := rf.logLastIndex
	var Done int32
	Done = 1
	// logs := []*Log{}
	// logs = append(logs, rf.logs[rf.preAppendLogIndex:]...)
	// fmt.Printf("index %v 开始发送日志 %v term %v\n", rf.me, command, rf.currentTerm)
	//开始复制日志
	for k, _ := range rf.peers {
		if rf.state != "L" {
			return
		}
		if k == rf.me {
			continue
		}
		i := rf.nextIndex[k] - 1
		if i <= 0 {
			time.Sleep(70 * time.Millisecond)
		}

		nextIndex := rf.nextIndex[k]
		args := AppendEntriesArgs{
			Term:         rf.currentTerm,
			LeaderId:     rf.me,
			PrevLogIndex: rf.nextIndex[k] - 1,
			PrevLogTerm:  rf.logs[i].Term,
			Entries:      rf.logs[nextIndex : lastIndex+1],
			LeaderCommit: rf.commitIndex,
		}
		fmt.Printf("&&&&&&&&&&& L向 %v 发送 日志 [%v:%v]\n", k, nextIndex, lastIndex)
		reply := AppendEntriesReply{}

		go rf.AppendEntriesRunTime(k, &Done, args, reply)
	}
	// Your code here (2B).
	ms1 := 100
	time.Sleep(time.Duration(ms1) * time.Millisecond)
	if int(Done) >= len(rf.peers)/2+1 {

		for i := rf.preCommitIndex + 1; i <= lastIndex; i++ {
			x := ApplyMsg{
				CommandValid: true,
				Command:      rf.logs[i].Command,
				CommandIndex: i,
			}
			rf.applyCh <- x
			fmt.Printf("*********{lindex %v commitindex %v log %v pre %v i %v \n",
				rf.me, lastIndex, rf.logs[i].Command, rf.preCommitIndex, i)
		}
		rf.preCommitIndex = lastIndex
		rf.commitIndex = lastIndex
	}
	rf.preAppendLogIndex = lastIndex
	rf.persist()

}