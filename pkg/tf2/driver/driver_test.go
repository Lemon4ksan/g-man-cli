// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	tr "github.com/lemon4ksan/g-man/pkg/steam/transport"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type mockCoordinatorProvider struct {
	sendFunc    func(ctx context.Context, appID, msgType uint32, msg proto.Message) error
	sendRawFunc func(ctx context.Context, appID, msgType uint32, payload []byte) error
}

func (m *mockCoordinatorProvider) Send(ctx context.Context, appID, msgType uint32, msg proto.Message) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, appID, msgType, msg)
	}

	return nil
}

func (m *mockCoordinatorProvider) SendRaw(ctx context.Context, appID, msgType uint32, payload []byte) error {
	if m.sendRawFunc != nil {
		return m.sendRawFunc(ctx, appID, msgType, payload)
	}

	return nil
}

func (m *mockCoordinatorProvider) Call(
	ctx context.Context,
	appID, msgType uint32,
	msg proto.Message,
	cb jobs.Callback[*protocol.GCPacket],
) error {
	if cb != nil {
		go cb(ctx, nil, nil)
	}

	return nil
}

func (m *mockCoordinatorProvider) CallRaw(
	ctx context.Context,
	appID, msgType uint32,
	payload []byte,
	cb jobs.Callback[*protocol.GCPacket],
) error {
	if cb != nil {
		go cb(ctx, nil, nil)
	}

	return nil
}

type mockServiceDoer struct {
	doFunc func(ctx context.Context, req *tr.Request) (*tr.Response, error)
}

func (m *mockServiceDoer) Do(ctx context.Context, req *tr.Request) (*tr.Response, error) {
	if m.doFunc != nil {
		return m.doFunc(ctx, req)
	}

	return nil, nil
}

type mockCommunityRequester struct {
	aoni.Requester
	sessionIDFunc func(baseURL string) string
}

func (m *mockCommunityRequester) SessionID(baseURL string) string {
	if m.sessionIDFunc != nil {
		return m.sessionIDFunc(baseURL)
	}

	return "mock-session-id"
}

type mockRestRequester struct {
	requestFunc func(ctx context.Context, method, path string, mods ...aoni.RequestModifier) (*http.Response, error)
}

func (m *mockRestRequester) Request(
	ctx context.Context,
	method, path string,
	mods ...aoni.RequestModifier,
) (*http.Response, error) {
	if m.requestFunc != nil {
		return m.requestFunc(ctx, method, path, mods...)
	}

	return nil, nil
}

type mockBackpackCache struct {
	items []*tf2.Item
}

func (m *mockBackpackCache) GetItems() []*tf2.Item {
	return m.items
}

func (m *mockBackpackCache) GetItem(id uint64) (*tf2.Item, bool) {
	for _, it := range m.items {
		if it.ID == id {
			return it, true
		}
	}

	return nil, false
}

func (m *mockBackpackCache) GetMaxSlots() int {
	return 3000
}

func getUnexportedField(target any, fieldName string) reflect.Value {
	val := reflect.ValueOf(target).Elem()
	field := val.FieldByName(fieldName)
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func setUnexportedField(target any, fieldName string, value any) {
	getUnexportedField(target, fieldName).Set(reflect.ValueOf(value))
}

func setFSMState(tf2Mod *tf2.TF2, state int) {
	fsmVal := getUnexportedField(tf2Mod, "fsm")
	fsmStruct := fsmVal.Elem()

	currentField := fsmStruct.FieldByName("current")
	if currentField.IsValid() {
		fsmCurrent := reflect.NewAt(currentField.Type(), unsafe.Pointer(currentField.UnsafeAddr())).Elem()
		fsmCurrent.Set(reflect.ValueOf(state).Convert(currentField.Type()))
	}
}

func setupDriver(
	t *testing.T,
) (*Driver, *tf2.TF2, *backpack.Backpack, *schema.Manager, *web.Manager, *mockCoordinatorProvider, *mockServiceDoer, *mockCommunityRequester) {
	logger := log.Discard
	busObj := bus.New()

	cfg := steam.DefaultConfig()
	cfg.Bus = busObj
	cfg.DisableSocket = true

	tf2Mod := tf2.New()
	bpMod := backpack.New()
	schemaMod := schema.NewManager(schema.DefaultConfig())
	webMod := web.New(web.DefaultConfig())

	client, err := steam.NewClient(cfg,
		steam.WithLogger(logger),
		steam.WithModule(tf2Mod),
		steam.WithModule(bpMod),
		steam.WithModule(schemaMod),
		steam.WithModule(webMod),
	)
	require.NoError(t, err)

	// Set event bus on modules
	tf2Mod.Bus = busObj
	bpMod.Bus = busObj
	schemaMod.Bus = busObj
	webMod.Bus = busObj

	gcMock := &mockCoordinatorProvider{}
	setUnexportedField(tf2Mod, "gc", gcMock)

	// Set state to Connected via FSM
	setFSMState(tf2Mod, 2)

	// Create and set SOCache
	soCache := tf2.NewSOCache(gcMock)
	setUnexportedField(tf2Mod, "cache", soCache)

	// Backpack module unexported fields:
	setUnexportedField(bpMod, "manager", schemaMod)
	setUnexportedField(bpMod, "tf2", tf2Mod)

	bpCache := &mockBackpackCache{}
	setUnexportedField(bpMod, "cache", bpCache)

	// Web module unexported fields:
	doerMock := &mockServiceDoer{}
	restMock := &mockRestRequester{}
	commMock := &mockCommunityRequester{
		Requester: restMock,
	}

	setUnexportedField(webMod, "web", doerMock)
	setUnexportedField(webMod, "community", commMock)

	// Setup default Schema
	rawSchema := &schema.Raw{}
	rawSchema.Schema.Qualities = map[string]int{
		"Unique":  6,
		"Normal":  0,
		"Genuine": 1,
		"Vintage": 3,
		"Strange": 11,
		"Unusual": 13,
	}
	rawSchema.Schema.Items = []*schema.Item{
		{
			Defindex:      5021,
			ItemName:      "Mann Co. Supply Crate Key",
			ItemClass:     "tool",
			UsedByClasses: []string{},
		},
		{
			Defindex:      5000,
			ItemName:      "Scout Weapon",
			ItemClass:     "tf_weapon_scattergun",
			CraftClass:    "weapon",
			UsedByClasses: []string{"Scout"},
		},
		{
			Defindex:      5001,
			ItemName:      "Cosmetic Item",
			ItemClass:     "tf_wearable",
			CraftClass:    "hat",
			UsedByClasses: []string{"Scout"},
		},
		{
			Defindex:      5002,
			ItemName:      "Taunt Item",
			ItemClass:     "tf_wearable_taunt",
			UsedByClasses: []string{},
		},
		{
			Defindex:      5003,
			ItemName:      "Supply Crate",
			ItemClass:     "supply_crate",
			UsedByClasses: []string{},
		},
	}
	sch := schema.New(rawSchema)
	setUnexportedField(schemaMod, "schema", sch)

	d := New(client)

	return d, tf2Mod, bpMod, schemaMod, webMod, gcMock, doerMock, commMock
}

func TestDriver_Metadata(t *testing.T) {
	d, _, _, _, _, _, _, _ := setupDriver(t)
	assert.Equal(t, uint32(440), d.AppID())
	assert.NotNil(t, d.InventoryProvider())
}

func TestDriver_GCStartStop(t *testing.T) {
	d, _, _, _, _, _, _, _ := setupDriver(t)
	assert.NoError(t, d.OnStartGC(context.Background()))
	assert.NoError(t, d.OnStopGC(context.Background()))
}

func TestDriver_GetInventory(t *testing.T) {
	d, tf2Mod, _, _, _, _, _, _ := setupDriver(t)

	soCache := tf2Mod.Cache()
	itemsMap := map[uint64]*tf2.Item{
		123: {
			ID:          123,
			OriginalID:  122,
			DefIndex:    5021,
			Quality:     6,
			Quantity:    1,
			IsTradable:  true,
			IsCraftable: true,
			CustomName:  "Test Name",
			CustomDesc:  "Test Desc",
			SKU:         "5021;6",
		},
	}
	setUnexportedField(soCache, "items", itemsMap)

	items, err := d.GetInventory(context.Background())
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, uint64(123), items[0].AssetID)
	assert.Equal(t, "Test Name", items[0].Attributes["custom_name"])
	assert.Equal(t, "5021;6", items[0].Attributes["sku"])
}

func TestDriver_SortInventory(t *testing.T) {
	d, _, _, _, _, _, _, _ := setupDriver(t)
	err := d.SortInventory(context.Background(), log.Discard)
	assert.NoError(t, err)
}

func TestDriver_RunMaintenance(t *testing.T) {
	d, tf2Mod, bpMod, _, _, gcMock, _, _ := setupDriver(t)

	// Test 1: GC not connected
	setFSMState(tf2Mod, 0)

	err := d.RunMaintenance(context.Background(), log.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active connection to TF2 Game Coordinator")

	// Set state back to Connected (2)
	setFSMState(tf2Mod, 2)

	// Setup bpMod cache and items (duplicate weapons)
	bpCache := &mockBackpackCache{
		items: []*tf2.Item{
			{
				ID:          101,
				DefIndex:    5000,
				Quality:     6,
				IsTradable:  true,
				IsCraftable: true,
			},
			{
				ID:          102,
				DefIndex:    5000,
				Quality:     6,
				IsTradable:  true,
				IsCraftable: true,
			},
		},
	}
	setUnexportedField(bpMod, "cache", bpCache)

	// Mock GC response to craft
	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		bpCache.items = nil

		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				BlueprintID:  0,
				CreatedItems: []uint64{201},
			})
		}()

		return nil
	}

	err = d.RunMaintenance(context.Background(), log.Discard)
	assert.NoError(t, err)
}

func TestDriver_ExecuteAction(t *testing.T) {
	d, tf2Mod, bpMod, _, webMod, gcMock, doerMock, commMock := setupDriver(t)
	bpCache := getUnexportedField(bpMod, "cache").Interface().(*mockBackpackCache)

	ctx := context.Background()

	// 1. "inventory" / "list-backpack" action
	soCache := tf2Mod.Cache()
	itemsMap := map[uint64]*tf2.Item{
		123: {
			ID:          123,
			OriginalID:  122,
			DefIndex:    5021,
			Quality:     6,
			Quantity:    1,
			IsTradable:  true,
			IsCraftable: true,
			SKU:         "5021;6",
		},
		124: {
			ID:          124,
			OriginalID:  122,
			DefIndex:    5000,
			Quality:     11,
			Quantity:    1,
			IsTradable:  true,
			IsCraftable: false,
			SKU:         "5000;11;uncraftable",
		},
		125: {
			ID:          125,
			OriginalID:  122,
			DefIndex:    5001,
			Quality:     13,
			Quantity:    1,
			IsTradable:  false,
			IsCraftable: true,
			SKU:         "5001;13",
		},
		126: {
			ID:          126,
			OriginalID:  122,
			DefIndex:    5002,
			Quality:     1,
			Quantity:    1,
			IsTradable:  true,
			IsCraftable: true,
			SKU:         "5002;1",
		},
		127: {
			ID:          127,
			OriginalID:  122,
			DefIndex:    5003,
			Quality:     3,
			Quantity:    1,
			IsTradable:  true,
			IsCraftable: true,
			SKU:         "5003;3",
		},
		128: {
			ID:          128,
			OriginalID:  122,
			DefIndex:    9999, // unknown
			Quality:     0,
			Quantity:    1,
			IsTradable:  true,
			IsCraftable: true,
			SKU:         "9999;0",
		},
	}
	setUnexportedField(soCache, "items", itemsMap)

	res, err := d.ExecuteAction(ctx, "inventory", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "BACKPACK INVENTORY")
	assert.Contains(t, res, "Mann Co. Supply Crate Key")
	assert.Contains(t, res, "Taunt Item")

	// 2. "sort-backpack" type=gc
	res, err = d.ExecuteAction(ctx, "sort-backpack", map[string]string{"type": "gc", "sort_type": "3"})
	assert.NoError(t, err)
	assert.Contains(t, res, "via Game Coordinator")

	// "sort-backpack" default
	res, err = d.ExecuteAction(ctx, "sort-backpack", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "via G-MAN continuous")

	// 3. "maintenance"
	res, err = d.ExecuteAction(ctx, "maintenance", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "maintenance")

	// 4. "craft-metal" scrap / reclaimed
	bpCache.items = []*tf2.Item{
		{ID: 11, DefIndex: 5000, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
		{ID: 12, DefIndex: 5000, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
		{ID: 13, DefIndex: 5000, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
	}
	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		bpCache.items = nil

		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				CreatedItems: []uint64{301},
			})
		}()

		return nil
	}
	res, err = d.ExecuteAction(ctx, "craft-metal", map[string]string{"type": "scrap"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Created item IDs: [301]")

	bpCache.items = []*tf2.Item{
		{ID: 21, DefIndex: 5001, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
		{ID: 22, DefIndex: 5001, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
		{ID: 23, DefIndex: 5001, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
	}
	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		bpCache.items = nil

		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				CreatedItems: []uint64{301},
			})
		}()

		return nil
	}
	res, err = d.ExecuteAction(ctx, "craft-metal", map[string]string{"type": "reclaimed"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Created item IDs: [301]")

	// 5. "delete-item"
	_, err = d.ExecuteAction(ctx, "delete-item", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "delete-item", map[string]string{"item_id": "abc"})
	assert.Error(t, err)

	res, err = d.ExecuteAction(ctx, "delete-item", map[string]string{"item_id": "123"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully deleted item 123")

	// 6. "use-item"
	_, err = d.ExecuteAction(ctx, "use-item", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "use-item", map[string]string{"item_id": "abc"})
	assert.Error(t, err)

	res, err = d.ExecuteAction(ctx, "use-item", map[string]string{"item_id": "123"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully used item 123")

	// 7. "acknowledge-all"
	res, err = d.ExecuteAction(ctx, "acknowledge-all", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully acknowledged all")

	// 8. "schema"
	res, err = d.ExecuteAction(ctx, "schema", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "Mann Co. Supply Crate Key")

	// 9. "condense-metal"
	bpCache.items = []*tf2.Item{
		{ID: 11, DefIndex: 5000, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
		{ID: 12, DefIndex: 5000, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
		{ID: 13, DefIndex: 5000, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
	}
	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		bpCache.items = nil

		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				CreatedItems: []uint64{301},
			})
		}()

		return nil
	}
	res, err = d.ExecuteAction(ctx, "condense-metal", nil)
	assert.NoError(t, err)
	assert.Equal(t, "1", res) // 1 condensation occurred from gc mock

	// 10. "make-change"
	_, err = d.ExecuteAction(ctx, "make-change", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "make-change", map[string]string{"target_defindex": "abc"})
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "make-change", map[string]string{"target_defindex": "5000"})
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "make-change", map[string]string{"target_defindex": "5000", "target_count": "abc"})
	assert.Error(t, err)

	bpCache.items = []*tf2.Item{
		{ID: 21, DefIndex: 5001, Quality: 6, Quantity: 1, IsTradable: true, IsCraftable: true},
	}
	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		bpCache.items = []*tf2.Item{
			{ID: 302, DefIndex: 5000, Quality: 6, Quantity: 1},
			{ID: 303, DefIndex: 5000, Quality: 6, Quantity: 1},
		}

		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				CreatedItems: []uint64{302, 303},
			})
		}()

		return nil
	}
	res, err = d.ExecuteAction(ctx, "make-change", map[string]string{"target_defindex": "5000", "target_count": "2"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully made change")

	// 11. "smelt-weapons"
	_, err = d.ExecuteAction(ctx, "smelt-weapons", nil)
	assert.Error(t, err)

	bpCache.items = []*tf2.Item{
		{ID: 101, DefIndex: 5000, Quality: 6, IsTradable: true, IsCraftable: true},
		{ID: 102, DefIndex: 5000, Quality: 6, IsTradable: true, IsCraftable: true},
	}
	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		bpCache.items = nil

		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				CreatedItems: []uint64{302, 303},
			})
		}()

		return nil
	}

	res, err = d.ExecuteAction(ctx, "smelt-weapons", map[string]string{"class": "Scout"})
	assert.NoError(t, err)
	assert.Contains(t, res, "[302,303]")

	// 12. "send-offer"
	_, err = d.ExecuteAction(ctx, "send-offer", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "send-offer", map[string]string{"offer_params": "{"}) // invalid json
	assert.Error(t, err)

	commMock.Requester.(*mockRestRequester).requestFunc = func(ctx context.Context, method, path string, mods ...aoni.RequestModifier) (*http.Response, error) {
		respJSON := `{"tradeofferid":"987654"}`

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(respJSON)),
			Header:     make(http.Header),
		}, nil
	}

	res, err = d.ExecuteAction(
		ctx,
		"send-offer",
		map[string]string{"offer_params": `{"partner_id":"76561197960287930"}`},
	)
	assert.NoError(t, err)
	assert.Equal(t, "987654", res)

	// 13. "accept-offer"
	_, err = d.ExecuteAction(ctx, "accept-offer", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "accept-offer", map[string]string{"offer_id": "abc"})
	assert.Error(t, err)

	doerMock.doFunc = func(ctx context.Context, req *tr.Request) (*tr.Response, error) {
		body := []byte(`{}`)
		return tr.NewResponse(io.NopCloser(bytes.NewReader(body)), nil), nil
	}
	res, err = d.ExecuteAction(ctx, "accept-offer", map[string]string{"offer_id": "987654"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully accepted offer")

	// 14. "decline-offer"
	_, err = d.ExecuteAction(ctx, "decline-offer", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "decline-offer", map[string]string{"offer_id": "abc"})
	assert.Error(t, err)

	res, err = d.ExecuteAction(ctx, "decline-offer", map[string]string{"offer_id": "987654"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully declined offer")

	// 15. "check-escrow"
	_, err = d.ExecuteAction(ctx, "check-escrow", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "check-escrow", map[string]string{"offer": "{"})
	assert.Error(t, err)

	commMock.Requester.(*mockRestRequester).requestFunc = func(ctx context.Context, method, path string, mods ...aoni.RequestModifier) (*http.Response, error) {
		html := `<html><body><script>var g_daysMyEscrow = 0; var g_daysTheirEscrow = 0;</script></body></html>`

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(html)),
			Header:     make(http.Header),
		}, nil
	}
	res, err = d.ExecuteAction(ctx, "check-escrow", map[string]string{"offer": `{"tradeofferid":"987654"}`})
	assert.NoError(t, err)
	assert.Equal(t, "false", res)

	// 16. "craft"
	_, err = d.ExecuteAction(ctx, "craft", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "craft", map[string]string{"recipe": "abc"})
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "craft", map[string]string{"recipe": "3"})
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "craft", map[string]string{"recipe": "3", "items": "{"})
	assert.Error(t, err)

	gcMock.sendRawFunc = func(ctx context.Context, appID, msgType uint32, payload []byte) error {
		go func() {
			time.Sleep(5 * time.Millisecond)
			tf2Mod.Bus.Publish(&tf2.CraftResponseEvent{
				CreatedItems: []uint64{401},
			})
		}()

		return nil
	}
	res, err = d.ExecuteAction(ctx, "craft", map[string]string{"recipe": "3", "items": "[101,102]"})
	assert.NoError(t, err)
	assert.Contains(t, res, "[401]")

	// 17. "get-partner-inventory"
	_, err = d.ExecuteAction(ctx, "get-partner-inventory", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "get-partner-inventory", map[string]string{"partner_id": "invalid"})
	assert.Error(t, err)

	commMock.Requester.(*mockRestRequester).requestFunc = func(ctx context.Context, method, path string, mods ...aoni.RequestModifier) (*http.Response, error) {
		inventoryJSON := `{
			"success": true,
			"total_inventory_count": 1,
			"assets": [
				{"assetid": "1001", "classid": "201", "instanceid": "301", "amount": "1"}
			],
			"descriptions": [
				{"classid": "201", "instanceid": "301", "market_hash_name": "5021;6", "tradable": 1}
			]
		}`

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(inventoryJSON)),
			Header:     make(http.Header),
		}, nil
	}
	res, err = d.ExecuteAction(ctx, "get-partner-inventory", map[string]string{"partner_id": "76561197960287930"})
	assert.NoError(t, err)
	assert.Contains(t, res, "1001")

	// 18. "active-offers"
	setUnexportedField(webMod, "receivedOffers", map[uint64]trading.OfferState{
		777: trading.OfferStateActive,
	})

	doerMock.doFunc = func(ctx context.Context, req *tr.Request) (*tr.Response, error) {
		body := []byte(`{
			"response": {
				"offer": {
					"tradeofferid": "777",
					"accountid_other": 12345,
					"message": "hi",
					"trade_offer_state": 2,
					"items_to_give": [],
					"items_to_receive": []
				}
			}
		}`)

		return tr.NewResponse(io.NopCloser(bytes.NewReader(body)), nil), nil
	}
	res, err = d.ExecuteAction(ctx, "active-offers", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "777")

	// 18b. "cancel-offer"
	_, err = d.ExecuteAction(ctx, "cancel-offer", nil)
	assert.Error(t, err)
	_, err = d.ExecuteAction(ctx, "cancel-offer", map[string]string{"offer_id": "abc"})
	assert.Error(t, err)

	doerMock.doFunc = func(ctx context.Context, req *tr.Request) (*tr.Response, error) {
		body := []byte(`{}`)
		return tr.NewResponse(io.NopCloser(bytes.NewReader(body)), nil), nil
	}
	res, err = d.ExecuteAction(ctx, "cancel-offer", map[string]string{"offer_id": "987654"})
	assert.NoError(t, err)
	assert.Contains(t, res, "Successfully cancelled offer")

	// 18c. "active-sent-offers"
	setUnexportedField(webMod, "sentOffers", map[uint64]trading.OfferState{
		888: trading.OfferStateActive,
	})

	doerMock.doFunc = func(ctx context.Context, req *tr.Request) (*tr.Response, error) {
		body := []byte(`{
			"response": {
				"offer": {
					"tradeofferid": "888",
					"accountid_other": 12345,
					"message": "sent",
					"trade_offer_state": 2,
					"items_to_give": [],
					"items_to_receive": []
				}
			}
		}`)

		return tr.NewResponse(io.NopCloser(bytes.NewReader(body)), nil), nil
	}
	res, err = d.ExecuteAction(ctx, "active-sent-offers", nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "888")

	// 19. Default/Unsupported
	_, err = d.ExecuteAction(ctx, "unsupported-action", nil)
	assert.Error(t, err)
}

func TestDriver_Actions(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg)
	require.NoError(t, err)

	d := New(client)
	actions := d.Actions()
	assert.NotEmpty(t, actions)

	// Verify inventory action is in the list
	foundInventory := false
	for _, act := range actions {
		if act.Name == "inventory" {
			foundInventory = true
			break
		}
	}

	assert.True(t, foundInventory, "Expected 'inventory' action to be present")
}

func TestExtractQuotedString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		ok       bool
	}{
		{`"hello"`, "hello", true},
		{`''world''`, "world", true},
		{`“curly”`, "curly", true},
		{`‘singlecurly’`, "singlecurly", true},
		{`no quotes`, "", false},
		{`"nested ''quotes''"`, "nested ''quotes''", true},
		{`''"nested double"''`, `"nested double"`, true},
	}

	for _, tc := range tests {
		res, ok := extractQuotedString(tc.input)
		assert.Equal(t, tc.ok, ok, "for input: %s", tc.input)

		if tc.ok {
			assert.Equal(t, tc.expected, res, "for input: %s", tc.input)
		}
	}
}
