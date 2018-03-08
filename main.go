package main

import (//依赖
  "bufio"
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "io"
  "log"
  //"net/http"
  "net"
  "os"
  "strconv"
  "sync"
  "time"

  "github.com/davecgh/go-spew/spew"
  //"github.com/gorilla/mux"
  "github.com/joho/godotenv"
)

//数据模型, 代表每一个块的数据模型
type Block struct {
  Index     int  //这个块在链中的位置
  Timestamp string // 块生成的时间戳
  BPM       int   //心跳数据 BPM 心率
  Hash      string   //这个块通过SHA256算法生成的散列值
  PrevHash  string //前一个块的SHA256散列值
}

/**
 * POST请求的handler稍微复杂， 定义POST请求的payload
 */
type Message struct {
  BPM int
}

//声明一个锁变量, 保持每次仅仅生成一个区块
var mutex = &sync.Mutex{}

//Blockchain is a series of validated Blocks
var Blockchain []Block

//bcServer handles incoming concurrent Blocks
var bcServer chan []Block

func main() {
  err := godotenv.Load()
  if err != nil {
    log.Fatal(err)
  }

  bcServer = make(chan []Block)

  //create genesis block
  t := time.Now()

  genesisBlock := Block{0, t.String(), 0, "", ""}
  spew.Dump(genesisBlock)
  newBlockchain := append(Blockchain, genesisBlock)
  replaceChain(newBlockchain)
  //start TCP and serve TCP server

  server, err := net.Listen("tcp", ":"+os.Getenv("ADDR"))

  if err != nil {
    log.Fatal(err)
  }

  defer server.Close()


  for {
    conn, err := server.Accept()
    if err != nil {
      log.Fatal(err)
    }
    go handleConn(conn)
  }

  // go func() {
  //   t := time.Now()
  //   genesisBlock := Block{0, t.String(), 0, "", ""}
  //   spew.Dump(genesisBlock)
  //   Blockchain = append(Blockchain, genesisBlock)
  // }()
  // log.Fatal(run())
}

func handleConn(conn net.Conn) {
  defer conn.Close()

  io.WriteString(conn, "Enter a new BPM:")

  scanner := bufio.NewScanner(conn)

  //take in BPM from stdin and add it to blockchain after conducting necessary validation

  go func() {
    for scanner.Scan() {
      bpm, err := strconv.Atoi(scanner.Text())

      if err != nil {
        log.Printf("%v not a number: %v", scanner.Text(), err)
        continue
      }

      newBlock := generateBlock(Blockchain[len(Blockchain)-1], bpm)

      // if err != nil {
      //   log.Printf(err)
      //   continue;
      // }

      if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
        newBlockchain := append(Blockchain, newBlock)
        replaceChain(newBlockchain)
      }

      bcServer <- Blockchain
      io.WriteString(conn, "\nEnter a new BPM:")
    }
  }()

  //simulate receiving broadcast
  go func() {
    for {
      time.Sleep(1 * time.Second)  //广播的间隔
      mutex.Lock()
      output, err := json.Marshal(Blockchain)

      if err != nil {
        log.Fatal(err)
      }

      mutex.Unlock()

      io.WriteString(conn, string(output))
    }
  }()

  for _ = range bcServer {
    spew.Dump(Blockchain, "\n")
  }
}


/**
 * Gorilla/mux 初始化Web服务
 * 其中的端口号是通过前面提到的 .env 来获得，再添加一些基本的配置参数，这个 web 服务就已经可以 listen and serve
 */
// func run() error {
//   mux := makeMuxRouter()
//   httpAddr := os.Getenv("ADDR")
//   log.Println("Listen on", os.Getenv("ADDR"))
//   s := &http.Server{
//     Addr:           ":" + httpAddr,
//     Handler :       mux,
//     ReadTimeout:    10 * time.Second,
//     WriteTimeout:   10 * time.Second,
//     MaxHeaderBytes: 1 << 20,
//   }
//
//   if err := s.ListenAndServe(); err != nil {
//     return err
//   }
//
//   return nil
// }

/**
 *alculateHash 函数接受一个块，通过块中的 Index，Timestamp，BPM，以及 PrevHash 值来计算出 SHA256 散列值
 */
func calculateHash(block Block) string {
  record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.BPM) + block.PrevHash
  h := sha256.New()
  h.Write([]byte(record))
  hashed := h.Sum(nil)

  return hex.EncodeToString(hashed)
}

/**
  * 区块生成函数
  *Index 是从给定的前一块的 Index 递增得出，时间戳是直接通过 time.Now 函数来获得的，Hash 值通过前面的 calculateHash 函数计算得出，PrevHash 则是给定的前一个块的 Hash 值
  */
func generateBlock(prevBlock Block, BPM int) Block {
  var newBlock Block

  t:= time.Now()
  newBlock.Index = prevBlock.Index + 1
  newBlock.Timestamp = t.String()
  newBlock.BPM = BPM
  newBlock.PrevHash = prevBlock.Hash
  newBlock.Hash = calculateHash(newBlock)

  return newBlock
}

/**
 * 校验块
 * 判断一个块是否被篡改。检查Index来看这个块是否正确递增，
 * 检查PrevHash与前一个块Hash是否一致
 * 通过calculateHash检测当前Hash
 */
func isBlockValid(curBlock, prevBlock Block) bool {
  if prevBlock.Index + 1 != curBlock.Index {
    return false
  }
  if prevBlock.Hash != curBlock.PrevHash {
    return false
  }
  if calculateHash(curBlock) != curBlock.Hash {
    return false
  }

  return true
}

//make sure the chain we're checking is longer than the current blockchain
func replaceChain(newBlocks []Block) {
  mutex.Lock()
  if len(newBlocks) > len(Blockchain) {
    Blockchain = newBlocks
  }
  mutex.Unlock()
}

/**
 * 定义不同的endpoint以及对应的handler
 *  例如, 对于 "/" 的GET请求，我们可以查看整个链
 *  ”/“ 的POST请求可以创建块
 */
// func makeMuxRouter() http.Handler {
//   muxRouter := mux.NewRouter()
//   muxRouter.HandleFunc("/", handleGetBlockchain).Methods("GET")
//   muxRouter.HandleFunc("/", handleWriteBlock).Methods("POST")
//   return muxRouter
// }

/**
 * GET请求的handler
 * 直接以 JSON 格式返回整个链，你可以在浏览器中访问 localhost:8080 或者 127.0.0.1:8080 来查看
 */
// func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
//   bytes, err := json.MarshalIndent(Blockchain, "","  ")
//   if err != nil {
//     http.Error(w, err.Error(), http.StatusInternalServerError)
//     return
//   }
//
//   io.WriteString(w, string(bytes))
// }

/*
 * POST 请求的handler
 */
// func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
//   w.Header().Set("Content-Type", "application/json")
//   var m Message
//
//   decoder := json.NewDecoder(r.Body)
//   if err := decoder.Decode(&m); err != nil {
//     responseWithJSON(w, r, http.StatusBadRequest, r.Body)
//     return
//   }
//
//   defer r.Body.Close()
//
//   mutex.Lock() //加锁
//   newBlock := generateBlock(Blockchain[len(Blockchain)-1], m.BPM)
//   mutex.Unlock() //移除锁
//
//   if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
//     newBlockchain := append(Blockchain, newBlock)
//     replaceChain(newBlockchain)
//     spew.Dump(Blockchain)
//   }
//
//   responseWithJSON(w, r, http.StatusCreated, newBlock)
// }


// func responseWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
//   response, err := json.MarshalIndent(payload, "", "  ")
//   if err != nil {
//     w.WriteHeader(http.StatusInternalServerError)
//     w.Write([]byte("HTTP 500: Internal Server Error"))
//     return
//   }
//   w.WriteHeader(code)
//   w.Write(response)
// }
