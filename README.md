# redis prefix analyse  
analyse which prefix used most  

unlike https://github.com/xueqiu/rdr we analyse only key and it's count  
so we can't get precise memory consume each prefix, but just get a brief report of keys in redis instead.  

from v1.3 we can calculate memory consume by command dump, it requires high bandwidths and may block redis,  
so use it on slave node when --size is on.  

it is a small and convenient util and do not need rdb file , just scan and detect prefix with trie tree  
pre-build release can be found at https://github.com/TimothyZhang023/redis_tool/releases 


```
redis_prefix_analyse-linux64-1.4 -h 127.0.0.1 -p 6379
more args can be found by --help
use  -readonly --size on slave node to get memory consume report
```

```
PREFIX] total visit count: 13744
--------------------Summary--------------------
    machine                                    189
    os-minute-                                 76
    redis-minute-                              13478
--------------------Summary<3>--------------------

*********************Detail**********************
    machine-minute-                            83
    machine-minute-connected_clients-          5
    machine-minute-instantaneous_              34
    machine-minute-keys-                       23
    machine-minute-used_memory-                21
    machine_ssdb-minute-ssdb_                  106
    machine_ssdb-minute-ssdb_file_size-        10
    machine_ssdb-minute-ssdb_keys-             7
    machine_ssdb-minute-ssdb_sst_file_size-    22
    machine_ssdb-minute-ssdb_used_             9
    os-minute-cpu_used_percent-                2
    os-minute-disk_                            30
    os-minute-disk_read_kbps-                  12
    os-minute-disk_used_percent-               16
    os-minute-disk_write_kbps-                 2
    os-minute-err                              22
    os-minute-load-                            2
    os-minute-net_                             18
    os-minute-tcp_connections-                 2
    redis-minute-connected_clients-            1325
    redis-minute-instantaneous_                3963
    redis-minute-keys                          2676
    redis-minute-ssdb_                         324
    redis-minute-ssdb_file_size-               35
    redis-minute-ssdb_keys-                    31
    redis-minute-ssdb_live_data_size-          34
    redis-minute-ssdb_sst_file_size-           40
    redis-minute-ssdb_used_                    78
    redis-minute-used_memory-                  1304
*********************Detail<29>**********************
```
