package main

import (
	"fmt"
	"time"
)

func worker(id int) {
	for i := 0; i < 50; i++ {
		fmt.Println("start")
		time.Sleep(500 * time.Millisecond)
	}
}

// func main() {
// 	a := []int{1, 3, 5, 8, 9, 10}

// 	i := binarySearchMax(a, 3)
// 	fmt.Println(i)
// }

func main() {
	// // 获取当前时间
	// currentTime := time.Now()

	// // 获取分钟、秒钟和毫秒
	// minute := currentTime.Minute()
	// second := currentTime.Second()
	// millisecond := currentTime.Nanosecond() / int(time.Millisecond)

	// // 输出结果
	// fmt.Printf("当前时间是 %02d 分 %02d 秒 %d 毫秒\n", minute, second, millisecond)

	x := []int{1, 2, 3, 4}

	fmt.Println(x[4:])
}

func binarySearchMax(arr []int, target int) int {
	left := 1
	right := len(arr) - 1
	for left <= right {
		mid := left + (right-left)/2
		fmt.Printf("l %v r %v mid %v\n", left, right, mid)
		if arr[mid] <= target {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	fmt.Printf("l %v r %v mid %v\n", left, right)
	// 检查是否越界
	if right < 0 {
		fmt.Printf("失败 l %v r %v\n", left, right)
		return -1 // 没有找到

	}
	fmt.Printf("成功 l %v r %v\n", left, right)
	return right // 返回最大值的索引
}
