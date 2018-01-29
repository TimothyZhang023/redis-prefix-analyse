package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/fvbock/trie"
	"gopkg.in/redis.v5"

	"errors"
	"sync"
)

var Quit = make(chan bool, 2)

var TYPENAMES = [6]string{"unknown", "string", "list", "set", "hash", "zset"}

var (
	host        = flag.String("h", "127.0.0.1", "host")
	port        = flag.Int("p", 6379, "port")
	action      = flag.String("action", "prefix", "操作 prefix...")
	scanPattern = flag.String("scan-pattern", "*", "scan匹配模式")
	sizeStat    = flag.Bool("size", false, "key size 统计")
	readOnly    = flag.Bool("readonly", false, "slave readonly")

	pipe = flag.Int64("pipe", 1000, "pipeline每次获取数量大小")

	prefixMaxDetect   = flag.Int("prefix-max-detect", 40, "最大前缀探测长度")
	prefixMinMembers  = flag.Int("prefix-min-members", 10, "前缀探测最低子元素个数,小于这个值则停止向后探测")
	prefixDetailLevel = flag.Int("prefix-detail-level", 2, "前缀探测详细等级,子元素个数超过这个值则继续增长prefix")
	prefixSamples     = flag.Int64("prefix-samples", 50000, "最低采样key的个数,注意实际可能因为min prefix参数使得key 大于这个值")
	prefixMinPrefix   = flag.Int64("prefix-min", 10, "最少前缀统计数. 当达到samples个数key之后如果采集到的prefix小于这个值则继续统计")
)

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error

	opt := redis.Options{
		ReadTimeout: -1,
		Addr:        fmt.Sprintf("%s:%d", *host, *port),
		ReadOnly:    *readOnly,
	}

	var client = redis.NewClient(&opt)
	defer client.Close()

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, os.Interrupt, os.Kill, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		quit := <-sc
		fmt.Println("Receive signal ", quit.String())
		Quit <- true
	}()

	fmt.Println(*action, "started")
	switch *action {
	case "prefix":
		go func() {
			var tc = NewTrieCounter()

			fmt.Printf("[PREFIX] try sample at least %d keys\n", *prefixSamples)

			err = ScanAndProcess(client, *pipe, *scanPattern, tc, func() bool {
				return !tc.scanEnd
			})

			fmt.Printf("[PREFIX] total samples count: %d\n", tc.keyCnt)
			tc.ProcessSamples()
			//tc.PrintAllStat()
			fmt.Printf("[PREFIX] process samples done, cntMapSummary:%d cntMapSummary:%d\n", len(tc.cntMapSummary), len(tc.cntMapDetail))

			err = ScanAndProcess(client, *pipe, *scanPattern, tc, func() bool {
				return true
			})

			fmt.Printf("[PREFIX] total visit count: %d\n", tc.keyCnt)

			tc.PrintAllStat()
			Quit <- true
		}()

		waitingForQuit()
	default:
		flag.Usage()
		panic(fmt.Sprintf("unknown command %s", *action))
	}

}

func waitingForQuit() {
	quit := false
	for {
		if quit {
			break
		}

		select {
		case <-Quit:
			Quit <- true
			quit = true
			break
		}
	}
}

type KeyProcess interface {
	Do(c *redis.Client, p []string) (err error)
}

type TrieCounter struct {
	t              *trie.Trie
	sampleEnd      bool
	scanEnd        bool
	keyCnt         int64
	samples        int64
	minPrefixCnt   int64
	cntMapSummary  map[string]int64
	SummaryKeys    []string
	cntMapDetail   map[string]int64
	DetailKeys     []string
	cntMapValueLen map[string]int64
	cntMapType     map[string]int
}

func NewTrieCounter() *TrieCounter {
	tc := TrieCounter{}
	tc.t = trie.NewTrie()
	tc.samples = *prefixSamples
	tc.minPrefixCnt = *prefixMinPrefix
	tc.cntMapSummary = make(map[string]int64, tc.minPrefixCnt)
	tc.cntMapDetail = make(map[string]int64, tc.minPrefixCnt)
	tc.cntMapValueLen = make(map[string]int64, tc.minPrefixCnt)
	tc.cntMapType = make(map[string]int, tc.minPrefixCnt)
	return &tc
}

func funcName(key string, prefixs []string) bool {
	for _, prefix := range prefixs {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func (b *TrieCounter) Do(client *redis.Client, keysD []string) (err error) {
	if b.sampleEnd {

		var keys []string
		for _, key := range keysD {
			b.keyCnt++
			if b.keyCnt%100000 == 0 {
				fmt.Printf("processed %d keys\n", b.keyCnt)
			}

			if funcName(key, b.SummaryKeys) {
				keys = append(keys, key)
				continue
			}

			if funcName(key, b.DetailKeys) {
				keys = append(keys, key)
				continue
			}
		}

		if len(keys) == 0 {
			return
		}

		typeMap := make(map[string]string, len(keys))
		sizeMap := make(map[string]int, len(keys))

		if *sizeStat {
			wg := &sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				pipeline := client.Pipeline()
				defer pipeline.Close()

				for _, key := range keys {
					pipeline.Type(key)
				}

				Cmders, _ := pipeline.Exec()
				for i, c := range Cmders {
					cmder := c.(*redis.StatusCmd)
					types, err := cmder.Result()
					if err != nil {
						typeMap[keys[i]] = "unknown"
						fmt.Fprintln(os.Stderr, err)
						continue
					}
					typeMap[keys[i]] = types
				}
			}()

			go func() {
				defer wg.Done()
				pipeline := client.Pipeline()
				defer pipeline.Close()

				for _, key := range keys {
					pipeline.Dump(key)
				}
				Cmders, _ := pipeline.Exec()
				for i, c := range Cmders {
					cmder := c.(*redis.StringCmd)
					val, err := cmder.Result()
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						sizeMap[keys[i]] = 0
						continue
					}
					sizeMap[keys[i]] = len(val)
				}
			}()
			wg.Wait()
		}

		for _, key := range keys {
			for prefix, keyCnt := range b.cntMapSummary {
				if strings.HasPrefix(key, prefix) {
					b.cntMapSummary[prefix] = keyCnt + 1
					b.recordType(typeMap, key, prefix)
					b.recordSize(sizeMap, key, prefix)
					break
				}
			}

			for prefix, keyCnt := range b.cntMapDetail {
				if strings.HasPrefix(key, prefix) {
					b.cntMapDetail[prefix] = keyCnt + 1
					b.recordType(typeMap, key, prefix)
					b.recordSize(sizeMap, key, prefix)
				}
			}
		}
	} else {
		for _, key := range keysD {
			b.keyCnt++
			if b.keyCnt > b.samples {
				b.scanEnd = true
				if b.ValidPrefixCnt() < b.minPrefixCnt {
					b.scanEnd = false
				}
			}
			b.t.Add(key)
		}
	}
	return nil
}
func (b *TrieCounter) recordSize(sizeMap map[string]int, key string, prefix string) {
	if _, ok := sizeMap[key]; ok {
		if _, ok := b.cntMapValueLen[prefix]; ok {
			b.cntMapValueLen[prefix] = b.cntMapValueLen[prefix] + int64(sizeMap[key])
		} else {
			b.cntMapValueLen[prefix] = int64(sizeMap[key])
		}
	}
}
func (b *TrieCounter) recordType(typeMap map[string]string, key string, prefix string) {
	if tname, ok := typeMap[key]; ok {
		for index, value := range TYPENAMES {
			if value == tname {
				b.cntMapType[prefix] = b.cntMapType[prefix] | 1<<uint(index)
				break
			}
		}
	}
}

func (b *TrieCounter) ValidPrefixCnt() (n int64) {
	for _, br := range b.t.Root.Branches {
		if !br.End && len(br.LeafValue) > 3 {
			n++
		}
	}
	return n
}

func (b *TrieCounter) ProcessSamples() {

	for fb, br := range b.t.Root.Branches {
		if !br.End && len(br.LeafValue) > 2 {
			prefix := string([]byte{fb}) + string(br.LeafValue)
			exists, _ := b.t.HasPrefixCount(prefix)
			if exists {
				b.cntMapSummary[prefix] = 0
			}

			members := b.t.PrefixMembersList(prefix)
			if len(members) > *prefixMinMembers {
				pf := b.processDetail(prefix, members, "")
				for _, p := range pf {
					b.cntMapDetail[p] = 0
				}
			}
		}
	}

	b.keyCnt = 0       //reset
	b.scanEnd = false  //reset
	b.sampleEnd = true //reset

	b.SummaryKeys = mapKeySet(b.cntMapSummary)
	b.DetailKeys = mapKeySet(b.cntMapDetail)

}

func (b *TrieCounter) processDetail(parent string, members []string, trim string) (pf []string) {
	tempTrie := trie.NewTrie()

	//log.Dev("** processDetail", parent, members)
	for _, item := range members {
		tempTrie.Add(strings.TrimPrefix(strings.TrimPrefix(item, parent), trim))
	}

	for fb, br := range tempTrie.Root.Branches {
		prefix := string([]byte{fb}) + string(br.LeafValue)
		//log.Dev("++ find path", br.End, parent, string([]byte{fb}), string(br.LeafValue), len(br.LeafValue))
		if !br.End && len(br.LeafValue) > 0 {
			exists, _ := tempTrie.HasPrefixCount(prefix)
			if exists {
				if len(parent+prefix) < *prefixMaxDetect {
					//log.Dev("-- find new prefix", parent+prefix)
					pf = append(pf, parent+prefix)

					members := tempTrie.PrefixMembersList(prefix)
					if len(members) > *prefixMinMembers {
						tpf := b.processDetail(parent+prefix, members, prefix)
						//log.Dev("tpf", len(tpf), tpf)
						if len(tpf) > *prefixDetailLevel {
							pf = append(pf, tpf...)
						}
					}
				}
			}
		}
	}
	return pf
}

func (b *TrieCounter) PrintAllStat() {
	fmt.Println("-------------------------------------------Summary-----------------------------------------")
	fmt.Printf(" %-42s %-10s %-10s %-13s %-10s\n", "PREFIX", "count", "avg-size", "total(byte)", "type(psb)")

	for _, prefix := range b.SummaryKeys {
		cnt := b.cntMapSummary[prefix]
		total := b.cntMapValueLen[prefix]
		typeN := getTypeName(b.cntMapType[prefix])
		avg := int64(0)
		if cnt != 0 {
			avg = total / cnt
		}
		fmt.Printf(" %-42s %-10d %-10d %-13d %+v\n", prefix, cnt, avg, total, typeN)
	}
	fmt.Printf("-------------------------------------------Summary<%d>---------------------------------------\n",
		len(b.cntMapSummary))

	fmt.Println("")

	fmt.Println("*****************************************Detail*****************************************")
	fmt.Printf(" %-42s %-10s %-10s %-13s %-10s\n", "PREFIX", "count", "avg-size", "total(byte)", "type(psb)")
	for _, prefix := range b.DetailKeys {
		cnt := b.cntMapDetail[prefix]
		total := b.cntMapValueLen[prefix]
		typeN := getTypeName(b.cntMapType[prefix])
		avg := int64(0)
		if cnt != 0 {
			avg = total / cnt
		}
		fmt.Printf(" %-42s %-10d %-10d %-13d %+v\n", prefix, cnt, avg, total, typeN)
	}
	fmt.Printf("*****************************************Detail<%d>**************************************\n",
		len(b.cntMapDetail))
	fmt.Println("")
}

func mapKeySet(m map[string]int64) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func getTypeName(ti int) (types []string) {
	for i := 0; i < len(TYPENAMES); i++ {
		if 1 & (ti >> uint(i)) !=0 {
			types = append(types, TYPENAMES[i])
		}
	}
	return
}

func ScanAndProcess(client *redis.Client, count int64, pattern string, p KeyProcess, continueFunc func() bool) error {
	fmt.Println(client.String(), "scan keys for pattern :", pattern)

	cursor := uint64(0)

	var err error

	var keys []string
	var c uint64

	for {
		if !continueFunc() {
			err = errors.New("scan Task was cancelled manually")
			break
		}

		keys, c, err = client.Scan(cursor, pattern, count).Result()
		if err != nil {
			break
		}

		if len(keys) > 0 {
			err = p.Do(client, keys)
		}
		if err != nil {
			break
		}

		cursor = c
		if cursor == uint64(0) {
			break
		} else {
			continue
		}
	}

	return err
}
