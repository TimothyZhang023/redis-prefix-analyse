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
	"time"

	"github.com/fvbock/trie"
	"gopkg.in/redis.v5"

	"errors"
	"io"
)

var Quit = make(chan bool, 2)

var (
	ip          = flag.String("h", "127.0.0.1", "ip")
	port        = flag.Int("p", 6379, "port")
	action      = flag.String("action", "prefix", "操作 prefix...")
	scanPattern = flag.String("scan-pattern", "*", "scan匹配模式")

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
		ReadTimeout: time.Duration(2000) * time.Millisecond,
		DialTimeout: time.Duration(2000) * time.Millisecond,
		Addr:        fmt.Sprintf("%s:%d", *ip, *port),
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

	fmt.Println(*action, "started ")
	switch *action {
	case "prefix":
		go func() {
			var tc = NewTrieCounter()

			fmt.Printf("[PREFIX] try sample at least %d keys", *prefixSamples)

			err = ScanAndProcess(client, *scanPattern, tc, func() bool {
				return !tc.scanEnd
			})

			fmt.Printf("[PREFIX] total sample count: %d\n", tc.keyCnt)
			tc.Process()
			tc.PrintAllStat()

			err = ScanAndProcess(client, *scanPattern, tc, func() bool {
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

type TrieCounter struct {
	t             *trie.Trie
	sampleEnd     bool
	scanEnd       bool
	keyCnt        int64
	samples       int64
	minPrefixCnt  int64
	cntMapSummary map[string]int64
	cntMapDetail  map[string]int64
}

func NewTrieCounter() *TrieCounter {
	tc := TrieCounter{}
	tc.t = trie.NewTrie()
	tc.samples = *prefixSamples
	tc.minPrefixCnt = *prefixMinPrefix
	tc.cntMapSummary = make(map[string]int64, tc.minPrefixCnt)
	tc.cntMapDetail = make(map[string]int64, tc.minPrefixCnt)
	return &tc
}

func (b *TrieCounter) Write(p []byte) (n int, err error) {

	key := strings.Trim(string(p), "\n")
	if b.sampleEnd {
		b.keyCnt++
		if b.keyCnt%800000 == 0 {
			fmt.Printf("processed %d keys\n", b.keyCnt)
		}

		for prefix, v := range b.cntMapSummary {
			if strings.HasPrefix(key, prefix) {
				b.cntMapSummary[prefix] = v + 1
				break
			}
		}

		for prefix, v := range b.cntMapDetail {
			if strings.HasPrefix(key, prefix) {
				b.cntMapDetail[prefix] = v + 1
			}
		}

		return len(p), nil
	} else {
		b.keyCnt++
		if b.keyCnt > b.samples {
			b.scanEnd = true
			if b.ValidPrefixCnt() < b.minPrefixCnt {
				b.scanEnd = false
			}
		}

		b.t.Add(key)
		return len(p), nil
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

func (b *TrieCounter) Process() {

	for fb, br := range b.t.Root.Branches {
		if !br.End && len(br.LeafValue) > 2 {
			prefix := string([]byte{fb}) + string(br.LeafValue)
			exists, _ := b.t.HasPrefixCount(prefix)
			if exists {
				b.cntMapSummary[prefix] = 0
			}

			members := b.t.PrefixMembersList(prefix)
			if len(members) > *prefixMinMembers {
				pf := b.process2(prefix, members, "")
				for _, p := range pf {
					b.cntMapDetail[p] = 0
				}
			}
		}
	}

	b.keyCnt = 0       //reset
	b.scanEnd = false  //reset
	b.sampleEnd = true //reset
}

func (b *TrieCounter) process2(parent string, members []string, trim string) (pf []string) {
	tempTrie := trie.NewTrie()

	//log.Dev("** process2", parent, members)
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
						tpf := b.process2(parent+prefix, members, prefix)
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
	fmt.Println("---------------------Summary--------------------")
	for _, k := range mapKeySet(b.cntMapSummary) {
		fmt.Printf("|    %-42s %d\n", k, b.cntMapSummary[k])
	}
	fmt.Printf("---------------------Summary<%d>--------------------\n", len(b.cntMapSummary))
	fmt.Println("")
	fmt.Println("**********************Detail**********************")
	for _, k := range mapKeySet(b.cntMapDetail) {
		fmt.Printf("|    %-42s %d\n", k, b.cntMapDetail[k])
	}
	fmt.Printf("**********************Detail<%d>**********************\n", len(b.cntMapDetail))
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

func ScanAndProcess(client *redis.Client, pattern string, keyWriter io.Writer, continueFunc func() bool) error {
	fmt.Println(client.String(), "scan keys for pattern :", pattern)

	cursor := uint64(0)
	count := int64(100)

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

		for _, key := range keys {
			fmt.Fprintln(keyWriter, key)
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
