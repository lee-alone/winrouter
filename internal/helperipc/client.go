package helperipc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
)

type Client struct {
	AuthToken string
	Dial      func() (net.Conn, error)
}

func (c Client) Call(method string, params any, result any) error {
	return c.CallContext(context.Background(), method, params, result)
}

func (c Client) CallContext(ctx context.Context, method string, params any, result any) error {
	if c.Dial == nil {
		return errors.New("helper dialer is required")
	}
	var raw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw = data
	}
	request := Request{ID: time.Now().UTC().Format("20060102T150405.000000000"), Method: method, AuthToken: c.AuthToken, Params: raw}
	connection, err := c.Dial()
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeFrame(connection, request); err != nil {
		return err
	}
	data, err := readFrame(connection)
	if err != nil {
		return err
	}
	var response struct {
		ID     string          `json:"id"`
		OK     bool            `json:"ok"`
		Error  *RPCError       `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	if !response.OK {
		if response.Error == nil {
			return errors.New("helper operation failed")
		}
		return errors.New(response.Error.Code + ": " + response.Error.Message)
	}
	if result != nil && len(response.Result) > 0 {
		return json.Unmarshal(response.Result, result)
	}
	return nil
}
