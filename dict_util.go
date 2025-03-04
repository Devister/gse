// Copyright 2016 ego authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package gse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

var (
	// LoadNoFreq load not have freq dict word
	LoadNoFreq bool
	// MinTokenFreq load min freq token
	MinTokenFreq = 2
)

// Dictionary 返回分词器使用的词典
func (seg *Segmenter) Dictionary() *Dictionary {
	return seg.dict
}

// AddToken add new text to token
func (seg *Segmenter) AddToken(text string, frequency int, pos ...string) {
	var po string
	if len(pos) > 0 {
		po = pos[0]
	}

	words := splitTextToWords([]byte(text))
	token := Token{text: words, frequency: frequency, pos: po}

	seg.dict.addToken(token)
}

// AddTokenForce add new text to token and force
func (seg *Segmenter) AddTokenForce(text string, frequency int, pos ...string) {
	seg.AddToken(text, frequency, pos...)
	seg.CalcToken()
}

// LoadDict load the dictionary from the file
//
// The format of the dictionary is (one for each participle):
//	participle text, frequency, part of speech
//
// Can load multiple dictionary files, the file name separated by ","
// the front of the dictionary preferentially load the participle,
//	such as: "user_dictionary.txt,common_dictionary.txt"
// When a participle appears both in the user dictionary and
// in the `common dictionary`, the `user dictionary` is given priority.
//
// 从文件中载入词典
//
// 可以载入多个词典文件，文件名用 "," 分隔，排在前面的词典优先载入分词，比如:
// 	"用户词典.txt,通用词典.txt"
// 当一个分词既出现在用户词典也出现在 `通用词典` 中，则优先使用 `用户词典`。
//
// 词典的格式为（每个分词一行）：
//	分词文本 频率 词性
func (seg *Segmenter) LoadDict(files ...string) error {
	seg.dict = NewDict()

	var (
		dictDir  = path.Join(path.Dir(getCurrentFilePath()), "data")
		dictPath string
		// load     bool
	)

	if len(files) > 0 {
		dictFiles := DictPaths(dictDir, files[0])
		if len(dictFiles) > 0 {
			// load = true
			// files = dictFiles
			for i := 0; i < len(dictFiles); i++ {
				err := seg.Read(dictFiles[i])
				if err != nil {
					return err
				}
			}
		}
	}

	if len(files) == 0 {
		dictPath = path.Join(dictDir, "dict/dictionary.txt")
		// files = []string{dictPath}
		err := seg.Read(dictPath)
		if err != nil {
			return err
		}
	}

	// if files[0] != "" && files[0] != "en" && !load {
	// 	for _, file := range strings.Split(files[0], ",") {
	// 		// for _, file := range files {
	// 		err := seg.Read(file)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	seg.CalcToken()
	log.Println("Gse dictionary loaded finished.")

	return nil
}

// getCurrentFilePath get current file path
func getCurrentFilePath() string {
	_, filePath, _, _ := runtime.Caller(1)
	return filePath
}

// Read read the dict flie
func (seg *Segmenter) Read(file string) error {
	log.Printf("Load the gse dictionary: \"%s\" ", file)
	isJson := false
	if strings.Contains(file, ".json") {
		isJson = true
	}
	dictFile, err := os.Open(file)
	if err != nil {
		log.Printf("Could not load dictionaries: \"%s\", %v \n", file, err)
		return err
	}
	defer dictFile.Close()

	reader := bufio.NewReader(dictFile)
	var (
		text      string
		freqText  string
		frequency int
		pos       string
	)

	// 逐行读入分词
	line := 0
	size := 0
	tokenJson := &TokenJson{}
	for {
		line++
		data, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Printf("File '%v' line \"%v\" read error: %v, skip",
					file, line, err.Error())
				continue
			}
		}
		size = len(data)
		if isJson {
			if err := json.Unmarshal(data, tokenJson); err != nil {
				log.Printf("File '%v' line \"%v\" unmarshal error: %v, skip",
					file, line, err.Error())
				continue
			}

			text, freqText, pos = tokenJson.Text, tokenJson.Frequency, tokenJson.Pos
		} else {
			fsize, fsErr := fmt.Sscanln(string(data), &text, &freqText, &pos)
			//fsize, fsErr := fmt.Fscanln(reader, &text, &freqText, &pos)
			if fsErr != nil {
				if fsize > 0 {
					log.Printf("File '%v' line \"%v\" read error: %v, skip",
						file, line, fsErr.Error())
				} else {
					log.Printf("File '%v' line \"%v\" is empty, read error: %v, skip",
						file, line, fsErr.Error())
				}
			}
			size = fsize
		}

		if size == 0 {
			// 文件结束或错误行
			// break
			continue
		} else if size < 2 {
			if !LoadNoFreq {
				// 无效行
				continue
			} else {
				freqText = "2"
			}
		} else if size == 2 {
			// 没有词性标注时设为空字符串
			pos = ""
		}

		// 解析词频
		frequency, err = strconv.Atoi(freqText)
		if err != nil {
			continue
		}

		// 过滤频率太小的词
		if frequency < MinTokenFreq {
			continue
		}
		// 过滤, 降低词频
		if len([]rune(text)) < 2 {
			// continue
			frequency = 2
		}

		// 将分词添加到字典中
		words := splitTextToWords([]byte(text))
		token := Token{text: words, frequency: frequency, pos: pos}
		seg.dict.addToken(token)
	}

	return nil
}

// DictPaths get the dict's paths
func DictPaths(dictDir, filePath string) (files []string) {
	var dictPath string

	if filePath == "en" {
		return
	}

	if filePath == "zh" {
		dictPath = path.Join(dictDir, "dict/dictionary.txt")
		files = []string{dictPath}

		return
	}

	if filePath == "jp" {
		dictPath = path.Join(dictDir, "dict/jp/dict.txt")
		files = []string{dictPath}

		return
	}

	// if strings.Contains(filePath, ",") {
	fileName := strings.Split(filePath, ",")
	for i := 0; i < len(fileName); i++ {
		if fileName[i] == "jp" {
			dictPath = path.Join(dictDir, "dict/jp/dict.txt")
		}

		if fileName[i] == "zh" {
			dictPath = path.Join(dictDir, "dict/dictionary.txt")
		}

		// if str[i] == "ti" {
		// }

		dictName := fileName[i] != "en" && fileName[i] != "zh" &&
			fileName[i] != "jp" && fileName[i] != "ti"

		if dictName {
			dictPath = fileName[i]
		}

		if dictPath != "" {
			files = append(files, dictPath)
		}
	}
	// }
	log.Println("Dict files path: ", files)

	return
}

// IsJp is jp char return true
func IsJp(segText string) bool {
	for _, r := range segText {
		jp := unicode.Is(unicode.Scripts["Hiragana"], r) ||
			unicode.Is(unicode.Scripts["Katakana"], r)
		if jp {
			return true
		}
	}
	return false
}

// CalcToken calc the segmenter token
func (seg *Segmenter) CalcToken() {
	// 计算每个分词的路径值，路径值含义见 Token 结构体的注释
	logTotalFrequency := float32(math.Log2(float64(seg.dict.totalFrequency)))
	for i := range seg.dict.tokens {
		token := &seg.dict.tokens[i]
		token.distance = logTotalFrequency - float32(math.Log2(float64(token.frequency)))
	}

	// 对每个分词进行细致划分，用于搜索引擎模式，
	// 该模式用法见 Token 结构体的注释。
	for i := range seg.dict.tokens {
		token := &seg.dict.tokens[i]
		segments := seg.segmentWords(token.text, true)

		// 计算需要添加的子分词数目
		numTokensToAdd := 0
		for iToken := 0; iToken < len(segments); iToken++ {
			// if len(segments[iToken].token.text) > 1 {
			// 略去字元长度为一的分词
			// TODO: 这值得进一步推敲，特别是当字典中有英文复合词的时候
			if len(segments[iToken].token.text) > 0 {
				hasJp := false
				if len(segments[iToken].token.text) == 1 {
					segText := string(segments[iToken].token.text[0])
					hasJp = IsJp(segText)
				}

				if !hasJp {
					numTokensToAdd++
				}
			}
		}
		token.segments = make([]*Segment, numTokensToAdd)

		// 添加子分词
		iSegmentsToAdd := 0
		for iToken := 0; iToken < len(segments); iToken++ {
			// if len(segments[iToken].token.text) > 1 {
			if len(segments[iToken].token.text) > 0 {
				hasJp := false
				if len(segments[iToken].token.text) == 1 {
					segText := string(segments[iToken].token.text[0])
					hasJp = IsJp(segText)
				}

				if !hasJp {
					token.segments[iSegmentsToAdd] = &segments[iToken]
					iSegmentsToAdd++
				}
			}
		}
	}

}
