package peercoin

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
)

// PeercoinRPC is an interface to JSON-RPC Peercoind service.
type PeercoinRPC struct {
	*btc.BitcoinRPC
}

// NewPeercoinRPC returns new PeercoinRPC instance.
func NewPeercoinRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &PeercoinRPC{
		b.(*btc.BitcoinRPC),
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}

	return s, nil
}

// Initialize initializes PeercoinRPC instance.
func (b *PeercoinRPC) Initialize() error {
	/*
		chainName, err := b.GetChainInfoAndInitializeMempool(b)
		if err != nil {
			return err
		}
	*/
	chainInfo, err := b.GetChainInfo()
	if err != nil {
		return err
	}

	chainName := chainInfo.Chain

	glog.Info("Chain name ", chainName)
	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewPeercoinParser(params, b.ChainConfig)

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)
	return nil
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type GenericCmd struct {
	ID     int           `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params,omitempty"`
}

type GetTransactionResult struct {
	Error  Error `json:"error"`
	Result struct {
		RawTx
	} `json:"result"`
}

type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

type Vin struct {
	Coinbase    string     `json:"coinbase"`
	Stakebase   string     `json:"stakebase"`
	Txid        string     `json:"txid"`
	Vout        uint32     `json:"vout"`
	Tree        int8       `json:"tree"`
	Sequence    uint32     `json:"sequence"`
	AmountIn    float64    `json:"amountin"`
	BlockHeight uint32     `json:"blockheight"`
	BlockIndex  uint32     `json:"blockindex"`
	ScriptSig   *ScriptSig `json:"scriptsig"`
}

type ScriptPubKeyResult struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex,omitempty"`
	ReqSigs   int32    `json:"reqSigs,omitempty"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses,omitempty"`
	CommitAmt *float64 `json:"commitamt,omitempty"`
}

type Vout struct {
	Value        float64            `json:"value"`
	N            uint32             `json:"n"`
	Version      uint16             `json:"version"`
	ScriptPubKey ScriptPubKeyResult `json:"scriptPubKey"`
}

type RawTx struct {
	Hex           string `json:"hex"`
	Txid          string `json:"txid"`
	Version       int32  `json:"version"`
	LockTime      uint32 `json:"locktime"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	Expiry        uint32 `json:"expiry"`
	BlockIndex    uint32 `json:"blockindex,omitempty"`
	Confirmations int64  `json:"confirmations,omitempty"`
	Time          int64  `json:"time,omitempty"`
	Blocktime     int64  `json:"blocktime,omitempty"`
}

type ResGetInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
		Version         string      `json:"version"`
		ProtocolVersion json.Number `json:"protocolversion"`
		Blocks          int         `json:"blocks"`
		TimeOffset      float64     `json:"timeoffset"`
		Difficulty      float64     `json:"difficulty"`
		BestBlockHash   string      `json:"bestblockhash"`
		Testnet         bool        `json:"testnet"`
		Chain           string      `json:"chain"`
	} `json:"result"`
}

type ResGetNetworkInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
		Version         json.Number `json:"version"`
		Subversion      json.Number `json:"subversion"`
		ProtocolVersion json.Number `json:"protocolversion"`
		Timeoffset      float64     `json:"timeoffset"`
		Warnings        string      `json:"warnings"`
	} `json:"result"`
}

type CmdGetDifficulty struct {
	Method string `json:"method"`
}

type ResGetDifficulty struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
		DifficultyPOW float64 `json:"proof-of-work"`
		DifficultyPOS float64 `json:"proof-of-stake"`
	} `json:"result"`
}

// GetChainInfo returns information about the connected backend
func (b *PeercoinRPC) GetChainInfo() (*bchain.ChainInfo, error) {
	glog.V(1).Info("rpc: getblockchaininfo")

	resI := ResGetInfo{}
	err := b.Call(&btc.CmdGetBlockChainInfo{Method: "getblockchaininfo"}, &resI)
	if err != nil {
		return nil, err
	}
	if resI.Error != nil {
		return nil, resI.Error
	}
	glog.V(1).Info("rpc: getnetworkinfo")
	resNi := btc.ResGetNetworkInfo{}
	err = b.Call(&btc.CmdGetNetworkInfo{Method: "getnetworkinfo"}, &resNi)
	if err != nil {
		return nil, err
	}
	glog.V(1).Info("rpc: getdifficulty")
	resD := ResGetDifficulty{}
	err = b.Call(&CmdGetDifficulty{Method: "getdifficulty"}, &resD)
	if err != nil {
		return nil, err
	}
	rv := &bchain.ChainInfo{
		Bestblockhash:   resI.Result.BestBlockHash,
		Blocks:          resI.Result.Blocks,
		Difficulty:      fmt.Sprintf("PoW: %v, PoS: %v", resD.Result.DifficultyPOW, resD.Result.DifficultyPOS),
		Timeoffset:      resNi.Result.Timeoffset,
		Subversion:      string(resNi.Result.Subversion),
		Version:         string(resNi.Result.Version),
		ProtocolVersion: string(resNi.Result.ProtocolVersion),
	}
	if resI.Result.Chain == "test" {
		rv.Chain = "testnet"
	} else {
		rv.Chain = "livenet"
	}
	return rv, nil
}

// GetBlock returns block with given hash.
func (b *PeercoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var err error
	if hash == "" && height > 0 {
		hash, err = b.GetBlockHash(height)

		if err != nil {
			return nil, err
		}
	}
	block, err := b.GetBlockFull(hash)
	if err != nil {
		return nil, err
	}

	for _, tx := range block.Txs {
		for i := range tx.Vout {
			vout := &tx.Vout[i]
			// convert vout.JsonValue to big.Int and clear it, it is only temporary value used for unmarshal
			vout.ValueSat, err = b.Parser.AmountToBigInt(vout.JsonValue)
			if err != nil {
				return nil, err
			}
			vout.JsonValue = ""
		}
	}

	return block, err
}

func isErrBlockNotFound(err *bchain.RPCError) bool {
	return err.Message == "Block not found" ||
		err.Message == "Block height out of range"
}

type CmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		Verbosity int    `json:"verbosity"`
	} `json:"params"`
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *PeercoinRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (txinfo=false) ", hash)

	res := btc.ResGetBlockInfo{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbosity = 1
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if isErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}
	return &res.Result, nil
}

// GetBlockHash returns hash of block in best-block-chain at given height.
func (b *PeercoinRPC) GetBlockHash(height uint32) (string, error) {
	glog.V(1).Info("rpc: getblockhash ", height)

	res := btc.ResGetBlockHash{}
	req := btc.CmdGetBlockHash{Method: "getblockhash"}
	req.Params.Height = height
	err := b.Call(&req, &res)

	if err != nil {
		return "", errors.Annotatef(err, "height %v", height)
	}
	if res.Error != nil {
		if isErrBlockNotFound(res.Error) {
			return "", bchain.ErrBlockNotFound
		}
		return "", errors.Annotatef(res.Error, "height %v", height)
	}
	return res.Result, nil
}

// GetTransactionForMempool returns a transaction by the transaction ID.
// It could be optimized for mempool, i.e. without block time and confirmations
func (b *PeercoinRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return b.GetTransaction(txid)
}

// GetTransaction returns a transaction by the transaction ID.
func (b *PeercoinRPC) GetTransaction(txid string) (*bchain.Tx, error) {
	r, err := b.getRawTransaction(txid)
	if err != nil {
		return nil, err
	}
	tx, err := b.Parser.ParseTxFromJson(r)
	if err != nil {
		return nil, errors.Annotatef(err, "txid %v", txid)
	}
	for i := range tx.Vout {
		if tx.Vout[i].ScriptPubKey.Addresses == nil {
			tx.Vout[i].ScriptPubKey.Addresses = []string{}
		}
	}
	return tx, nil
}

// getRawTransaction returns json as returned by backend, with all coin specific data

func (b *PeercoinRPC) getRawTransaction(txid string) (json.RawMessage, error) {
	glog.V(1).Info("rpc: getrawtransaction ", txid)

	res := btc.ResGetRawTransaction{}
	req := cmdGetRawTransaction{Method: "getrawtransaction"}
	req.Params.Txid = txid
	req.Params.Verbose = 1
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "txid %v", txid)
	}
	if res.Error != nil {
		if btc.IsMissingTx(res.Error) {
			return nil, bchain.ErrTxNotFound
		}
		return nil, errors.Annotatef(res.Error, "txid %v", txid)
	}
	return res.Result, nil
}

type cmdGetRawTransaction struct {
	Method string `json:"method"`
	Params struct {
		Txid    string `json:"txid"`
		Verbose int    `json:"verbose"`
	} `json:"params"`
}

// GetTransactionSpecific returns the json raw message for the tx identified by
// the provided txid.
func (b *PeercoinRPC) GetTransactionSpecific(tx *bchain.Tx) (json.RawMessage, error) {
	return b.getRawTransaction(tx.Txid)
}

/*
// GetTransactionSpecific returns json as returned by backend, with all coin specific data
func (b *PeercoinRPC) GetTransactionSpecific(txid string) (json.RawMessage, error) {
	r, err := b.BitcoinRPC.GetTransactionSpecific(txid)
	if err != nil {
		return r, err
	}
	// Peercoind getrawtransaction returns multiple "time" fields with different values.
	// We need to ensure the latter is removed.
	result, err := removeDuplicateJSONKeys(r)
	if err != nil {
		return nil, errors.Annotatef(err, "txid %v", txid)
	}
	return result, nil
}
*/
