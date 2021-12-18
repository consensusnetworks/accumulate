package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	api2 "github.com/AccumulateNetwork/accumulate/internal/api/v2"
	"github.com/AccumulateNetwork/accumulate/internal/url"
	"github.com/AccumulateNetwork/jsonrpc2/v15"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Create and get token txs",
	Run: func(cmd *cobra.Command, args []string) {
		var out string
		var err error
		if len(args) > 0 {
			switch arg := args[0]; arg {
			case "get":
				if len(args) > 1 {
					out, err = GetTX(args[1])
				} else {
					fmt.Println("Usage:")
					PrintTXGet()
				}
			case "history":
				if len(args) > 3 {
					out, err = GetTXHistory(args[1], args[2], args[3])
				} else {
					fmt.Println("Usage:")
					PrintTXHistoryGet()
				}
			case "create":
				if len(args) > 3 {
					out, err = CreateTX(args[1], args[2:])
				} else {
					fmt.Println("Usage:")
					PrintTXCreate()
				}
			default:
				fmt.Println("Usage:")
				PrintTX()
			}
		} else {
			fmt.Println("Usage:")
			PrintTX()
		}
		printOutput(cmd, out, err)
	},
}

var (
	TxWait      time.Duration
	TxWaitSynth time.Duration
)

func init() {
	txCmd.Flags().DurationVarP(&TxWait, "wait", "w", 0, "Wait for the transaction to complete")
	txCmd.Flags().DurationVar(&TxWaitSynth, "wait-synth", 0, "Wait for synthetic transactions to complete")
}

func PrintTXGet() {
	fmt.Println("  accumulate tx get [txid]			Get token transaction by txid")
}

func PrintTXCreate() {
	fmt.Println("  accumulate tx create [from] [to] [amount]	Create new token tx")
}

func PrintTXHistoryGet() {
	fmt.Println("  accumulate tx history [url] [starting transaction number] [ending transaction number]	Get transaction history")
}

func PrintTX() {
	PrintTXGet()
	PrintTXCreate()
	PrintTXHistoryGet()
}

func getTX(hash []byte, wait time.Duration) (*api2.QueryResponse, error) {
	var res api2.QueryResponse
	var err error

	params := new(api2.TxnQuery)
	params.Txid = hash

	if wait > 0 {
		params.Wait = wait
	}

	data, err := json.Marshal(params)
	jsondata := json.RawMessage(data)
	if err != nil {
		return nil, err
	}

	err = Client.RequestV2(context.Background(), "query-tx", jsondata, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func GetTX(hash string) (string, error) {
	txid, err := hex.DecodeString(hash)
	if err != nil {
		return "", err
	}

	t := Client.Timeout
	defer func() { Client.Timeout = t }()

	if TxWait > 0 {
		Client.Timeout = TxWait * 2
	}

	res, err := getTX(txid, TxWait)
	if err != nil {
		var rpcErr jsonrpc2.Error
		if errors.As(err, &rpcErr) {
			return PrintJsonRpcError(err)
		}
		return "", err
	}

	out, err := PrintQueryResponseV2(res)
	if err != nil {
		return "", err
	}

	if TxWaitSynth == 0 || len(res.SyntheticTxids) == 0 {
		return out, nil
	}

	if TxWaitSynth > 0 {
		Client.Timeout = TxWaitSynth * 2
	}

	errg := new(errgroup.Group)
	for _, txid := range res.SyntheticTxids {
		txid := txid // Do not capture the loop variable in the closure
		errg.Go(func() error {
			res, err := getTX(txid[:], TxWaitSynth)
			if err != nil {
				return err
			}

			o, err := PrintQueryResponseV2(res)
			if err != nil {
				return err
			}

			if WantJsonOutput {
				out += "\n"
			}
			out += o
			return nil
		})
	}
	err = errg.Wait()
	if err != nil {
		var rpcErr jsonrpc2.Error
		if errors.As(err, &rpcErr) {
			return PrintJsonRpcError(err)
		}
		return PrintJsonRpcError(err)
	}

	return out, nil
}

func GetTXHistory(accountUrl string, s string, e string) (string, error) {

	var res api2.QueryMultiResponse
	start, err := strconv.Atoi(s)
	if err != nil {
		return "", err
	}
	end, err := strconv.Atoi(e)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(accountUrl)
	if err != nil {
		return "", err
	}

	params := new(api2.TxHistoryQuery)
	params.UrlQuery.Url = u.String()
	params.QueryPagination.Start = uint64(start)
	params.QueryPagination.Count = uint64(end)

	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	if err := Client.RequestV2(context.Background(), "query-tx-history", json.RawMessage(data), &res); err != nil {
		return PrintJsonRpcError(err)
	}

	if WantJsonOutput {
		data, err := json.Marshal(res)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var out string
	out += fmt.Sprintf("\n\tTrasaction History Start: %d\t Count: %d\t Total: %d\n", res.Start, res.Count, res.Total)
	for i := range res.Items {
		s, err := PrintQueryResponseV2(res.Items[i])
		if err != nil {
			return "", err
		}
		out += s
	}

	return out, err
}

func CreateTX(sender string, args []string) (string, error) {
	//sender string, receiver string, amount string
	u, err := url.Parse(sender)
	if err != nil {
		return "", err
	}

	args, si, pk, err := prepareSigner(u, args)

	if len(args) < 2 {
		PrintTXCreate()
		return "", fmt.Errorf("invalid number of arguments for tx create")
	}

	u2, err := url.Parse(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid receiver url %s, %v", args[0], err)
	}
	amount := args[1]

	tokenSend := new(api2.TokenSend)
	tokenSend.From = u.String()

	amt, err := strconv.ParseFloat(amount, 64)
	r := api2.TokenDeposit{}
	r.Amount = uint64(amt * 1e8)
	r.Url = u2.String()
	tokenSend.To = append(tokenSend.To, r)

	res, err := dispatchTxRequest("send-tokens", &tokenSend, u, si, pk)
	if err != nil {
		return "", err
	}
	return ActionResponseFrom(res).Print()
}
