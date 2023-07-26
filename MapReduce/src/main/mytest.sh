#!/bin/bash

# 删除文件
rm -f mr-*

rm -f wc.so

# 编译插件
go build -buildmode=plugin ../mrapps/wc.go

# 运行协调器
go run mrcoordinator.go pg-*.txt &

