# redis_tool
redis prefix analyse
analyse which prefix used most

unlike https://github.com/xueqiu/rdr we analyse only key and it's count  
so we can't get memory consume each prefix, but just get a bref report of keys in redis instead.  

it is a small and convenient util and do not need rdb file , just scan and detect prefix with trie tree  
pre-build release can be found at https://github.com/TimothyZhang023/redis_tool/releases 


```
redis_tool-linux64-1.2 -h 127.0.0.1 -p 6379
more args can be found by --help
```

```
2018/01/25 17:24:55 [PREFIX] total visit count: 13744
2018/01/25 17:24:55 ---------------------Summary--------------------
2018/01/25 17:24:55 |    machine                                    189
2018/01/25 17:24:55 |    os-minute-                                 76
2018/01/25 17:24:55 |    redis-minute-                              13478
2018/01/25 17:24:55 ---------------------Summary<3>--------------------

2018/01/25 17:24:55 **********************Detail**********************
2018/01/25 17:24:55 |    machine-minute-                            83
2018/01/25 17:24:55 |    machine-minute-connected_clients-          5
2018/01/25 17:24:55 |    machine-minute-instantaneous_              34
2018/01/25 17:24:55 |    machine-minute-keys-                       23
2018/01/25 17:24:55 |    machine-minute-used_memory-                21
2018/01/25 17:24:55 |    machine_ssdb-minute-ssdb_                  106
2018/01/25 17:24:55 |    machine_ssdb-minute-ssdb_file_size-        10
2018/01/25 17:24:55 |    machine_ssdb-minute-ssdb_keys-             7
2018/01/25 17:24:55 |    machine_ssdb-minute-ssdb_sst_file_size-    22
2018/01/25 17:24:55 |    machine_ssdb-minute-ssdb_used_             9
2018/01/25 17:24:55 |    os-minute-cpu_used_percent-                2
2018/01/25 17:24:55 |    os-minute-disk_                            30
2018/01/25 17:24:55 |    os-minute-disk_read_kbps-                  12
2018/01/25 17:24:55 |    os-minute-disk_used_percent-               16
2018/01/25 17:24:55 |    os-minute-disk_write_kbps-                 2
2018/01/25 17:24:55 |    os-minute-err                              22
2018/01/25 17:24:55 |    os-minute-load-                            2
2018/01/25 17:24:55 |    os-minute-net_                             18
2018/01/25 17:24:55 |    os-minute-tcp_connections-                 2
2018/01/25 17:24:55 |    redis-minute-connected_clients-            1325
2018/01/25 17:24:55 |    redis-minute-instantaneous_                3963
2018/01/25 17:24:55 |    redis-minute-keys                          2676
2018/01/25 17:24:55 |    redis-minute-ssdb_                         324
2018/01/25 17:24:55 |    redis-minute-ssdb_file_size-               35
2018/01/25 17:24:55 |    redis-minute-ssdb_keys-                    31
2018/01/25 17:24:55 |    redis-minute-ssdb_live_data_size-          34
2018/01/25 17:24:55 |    redis-minute-ssdb_sst_file_size-           40
2018/01/25 17:24:55 |    redis-minute-ssdb_used_                    78
2018/01/25 17:24:55 |    redis-minute-used_memory-                  1304
2018/01/25 17:24:55 **********************Detail<29>**********************
```
