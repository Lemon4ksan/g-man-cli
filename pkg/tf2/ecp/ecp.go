// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

// Package ecp provides encoding and decoding of Team Fortress 2 items into
// compact, chat-friendly Easy-Copy-Paste (ECP) command strings.
//
// ECP Format Lore:
//   - Swaps standard words for compressed aliases (e.g. "Australium" -> "Aus", "Killstreak" -> "Ks").
//   - Replaces spaces and delimiters with underscores.
//   - Optionally stylizes characters using Unicode mathematical bold runes to avoid steam detection.
package ecp

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Sentinel errors for ECP parsing and generation.
var (
	ErrEmptyInputString      = errors.New("input string length is 0")
	ErrECPGenerationFailed   = errors.New("failed to generate ECP string")
	ErrUnknownCustomerIntent = errors.New("could not decide customer intent from ECP string")
	ErrItemNameNotFoundInECP = errors.New("the item name was not found in the ECP map")
)

var (
	nativeToBold = make(map[rune]rune)
	boldToNative = make(map[rune]rune)
)

func init() {
	nRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	bRunes := []rune("𝗮𝗯𝗰𝗱𝗲𝗳𝗴𝗵𝗶𝗷𝗸𝗹𝗺𝗻𝗼𝗽𝗾𝗿𝘀𝘁𝘂𝘃𝘄𝗫𝘆𝘇𝗔𝗕𝗖𝗗𝗘𝗙𝗚𝗛𝗜𝗝𝗞𝗟𝗠𝗡𝗢𝗣𝗤𝗥𝗦𝗧𝗨𝗩𝗪𝗫𝗬𝗭𝟬𝟭𝟮𝟯𝟰𝟱𝟲𝟳𝟴𝟵")

	for i := 0; i < len(nRunes) && i < len(bRunes); i++ {
		nativeToBold[nRunes[i]] = bRunes[i]
		boldToNative[bRunes[i]] = nRunes[i]
	}
}

var keyWordReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile("(?i)Australium"), "Aus"},
	{regexp.MustCompile("(?i)Killstreak"), "Ks"},
	{regexp.MustCompile("(?i)Specialized"), "Spec"},
	{regexp.MustCompile("(?i)Professional"), "Pro"},
	{regexp.MustCompile("(?i)'s"), "s"},
}

// MappedValue holds an item's canonical name alongside its generated ECP string variants.
type MappedValue struct {
	Key   string
	Value []string
}

// DecodedECP represents the decoded output of a parsed ECP string command.
type DecodedECP struct {
	OriginalItemName string
	DecodedIntent    string // "buy" or "sell"
}

// EasyCopyPaste handles stateful encoding and decoding of TF2 item names and intents.
type EasyCopyPaste struct {
	mu           sync.RWMutex
	useBoldChars bool
	useWordSwap  bool
	delimiters   []rune
	mappedItems  map[string][]string
}

// New creates a new [EasyCopyPaste] encoder/decoder instance.
func New() *EasyCopyPaste {
	return &EasyCopyPaste{
		delimiters:  []rune{' ', '\'', '-', '/', '.', '#', '!', ':', '(', ')', ','},
		mappedItems: make(map[string][]string),
	}
}

// SetUseBoldChars toggles Unicode mathematical bold character conversion.
func (e *EasyCopyPaste) SetUseBoldChars(val bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.useBoldChars = val
}

// SetUseWordSwap toggles keyword abbreviation replacement ("Australium" -> "Aus").
func (e *EasyCopyPaste) SetUseWordSwap(val bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.useWordSwap = val
}

// ToEcpString encodes an item name and bot trade intent into a customer-facing ECP string.
func (e *EasyCopyPaste) ToEcpString(itemOriginalName, botSideIntent string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(itemOriginalName) == 0 {
		return "", ErrEmptyInputString
	}

	customerSideIntent := "buy"
	if botSideIntent == "buy" {
		customerSideIntent = "sell"
	}

	mappedEcpEntry := e.mapString(itemOriginalName)
	if len(mappedEcpEntry.Value) == 0 {
		return "", ErrECPGenerationFailed
	}

	finalEcpStr := mappedEcpEntry.Value[0]
	if e.useWordSwap {
		for _, ecpStrEntry := range mappedEcpEntry.Value {
			if len(ecpStrEntry) < len(finalEcpStr) {
				finalEcpStr = ecpStrEntry
			}
		}
	}

	nativeEcpString := customerSideIntent + "_" + finalEcpStr
	if e.useBoldChars {
		return e.SwapToBoldChars(nativeEcpString), nil
	}

	return nativeEcpString, nil
}

// ReverseEcpString decodes an ECP command string back into its original item name and intent.
func (e *EasyCopyPaste) ReverseEcpString(escaped string) (*DecodedECP, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(escaped) == 0 {
		return nil, ErrEmptyInputString
	}

	nativeStr := e.SwapToNativeChars(escaped)
	customerIntent := ""

	if strings.HasPrefix(nativeStr, "sell_") {
		customerIntent = "sell"
	} else if strings.HasPrefix(nativeStr, "buy_") {
		customerIntent = "buy"
	}

	if customerIntent == "" {
		return nil, ErrUnknownCustomerIntent
	}

	intentClearedEcpStr := strings.Replace(nativeStr, customerIntent+"_", "", 1)

	itemMappedOriginalName, ok := e.findMappedValue(intentClearedEcpStr)
	if !ok {
		return nil, ErrItemNameNotFoundInECP
	}

	return &DecodedECP{
		OriginalItemName: itemMappedOriginalName.Key,
		DecodedIntent:    customerIntent,
	}, nil
}

// SwapToBoldChars converts standard ASCII characters into Unicode mathematical bold runes.
func (e *EasyCopyPaste) SwapToBoldChars(str string) string {
	var sb strings.Builder
	for _, r := range str {
		if b, ok := nativeToBold[r]; ok {
			sb.WriteRune(b)
		} else {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

// SwapToNativeChars converts Unicode mathematical bold runes back to standard ASCII runes.
func (e *EasyCopyPaste) SwapToNativeChars(str string) string {
	var sb strings.Builder
	for _, r := range str {
		if n, ok := boldToNative[r]; ok {
			sb.WriteRune(n)
		} else {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

func (e *EasyCopyPaste) swapPreMappedKeywords(ecpString string) string {
	result := ecpString
	for _, rep := range keyWordReplacements {
		result = rep.pattern.ReplaceAllString(result, rep.replacement)
	}

	return result
}

func (e *EasyCopyPaste) constructEcpCharSequence(originalItemName string) string {
	runes := []rune(originalItemName)
	result := make([]rune, 0, len(runes))

	for i, selectedChar := range runes {
		if e.isDelimiter(selectedChar) {
			if (i+1 < len(runes) && (runes[i+1] == ' ' || e.isDelimiter(runes[i+1]))) || i+1 == len(runes) {
				continue
			}

			result = append(result, '_')
		} else {
			result = append(result, selectedChar)
		}
	}

	return string(result)
}

func (e *EasyCopyPaste) isDelimiter(r rune) bool {
	return slices.Contains(e.delimiters, r)
}

func (e *EasyCopyPaste) findMappedValue(str string) (MappedValue, bool) {
	lowerCaseStr := strings.ToLower(str)
	for key, value := range e.mappedItems {
		if strings.ToLower(key) == lowerCaseStr {
			return MappedValue{Key: key, Value: value}, true
		}

		for _, entry := range value {
			if strings.ToLower(entry) == lowerCaseStr {
				return MappedValue{Key: key, Value: value}, true
			}
		}
	}

	return MappedValue{}, false
}

func (e *EasyCopyPaste) mapString(itemName string) MappedValue {
	if found, ok := e.findMappedValue(itemName); ok {
		return found
	}

	ecpFormatSet := make(map[string]struct{})
	ecpFormatSet[e.constructEcpCharSequence(itemName)] = struct{}{}
	ecpFormatSet[e.constructEcpCharSequence(e.swapPreMappedKeywords(itemName))] = struct{}{}
	ecpFormatSet[e.swapPreMappedKeywords(e.constructEcpCharSequence(itemName))] = struct{}{}

	ecpFormatDistinctArray := make([]string, 0, len(ecpFormatSet))
	for k := range ecpFormatSet {
		ecpFormatDistinctArray = append(ecpFormatDistinctArray, k)
	}

	e.mappedItems[itemName] = ecpFormatDistinctArray

	return MappedValue{Key: itemName, Value: ecpFormatDistinctArray}
}
