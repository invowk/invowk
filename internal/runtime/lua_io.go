// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	luart "github.com/arnodel/golua/runtime"
)

const luaFileClosedMsg = "file is closed"

type (
	luaIOBridge struct {
		r             *luart.Runtime
		pathValidator virtualPathValidator
		workDir       string
		defaultInput  *luaFile
		defaultOutput *luaFile
		files         map[*luart.Table]*luaFile
		funcs         []*luart.GoFunction
	}

	luaFile struct {
		name     string
		reader   *bufio.Reader
		writer   io.Writer
		file     *os.File
		closable bool
		closed   bool
	}

	luaRequireBridge struct {
		scriptBasePath string
		loaded         map[string]luart.Value
		loading        map[string]bool
	}
)

func installLuaIOBridge(
	r *luart.Runtime,
	pathValidator virtualPathValidator,
	workDir string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) []*luart.GoFunction {
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	bridge := &luaIOBridge{
		r:             r,
		pathValidator: pathValidator,
		workDir:       workDir,
		defaultInput:  luaReadFile("stdin", stdin),
		defaultOutput: luaWriteFile("stdout", stdout),
		files:         make(map[*luart.Table]*luaFile),
	}
	ioTable := luart.NewTable()
	r.SetTable(ioTable, luart.StringValue("stdin"), bridge.fileValue(bridge.defaultInput))
	r.SetTable(ioTable, luart.StringValue("stdout"), bridge.fileValue(bridge.defaultOutput))
	r.SetTable(ioTable, luart.StringValue("stderr"), bridge.fileValue(luaWriteFile("stderr", stderr)))
	bridge.addFunc(ioTable, "open", bridge.openFunc(), 2, false)
	bridge.addFunc(ioTable, "lines", bridge.linesFunc(), 1, true)
	bridge.addFunc(ioTable, "read", bridge.readFunc(), 0, true)
	bridge.addFunc(ioTable, "write", bridge.writeFunc(), 0, true)
	bridge.addFunc(ioTable, "flush", bridge.flushFunc(), 0, false)
	bridge.addFunc(ioTable, "input", bridge.inputFunc(), 1, false)
	bridge.addFunc(ioTable, "output", bridge.outputFunc(), 1, false)
	bridge.addFunc(ioTable, "type", bridge.typeFunc(), 1, false)
	r.SetEnv(r.GlobalEnv(), "io", luart.TableValue(ioTable))
	return bridge.funcs
}

func (b *luaIOBridge) addFunc(table *luart.Table, name string, fn luart.GoFunctionFunc, nArgs int, hasEtc bool) {
	b.funcs = append(b.funcs, b.r.SetEnvGoFunc(table, name, fn, nArgs, hasEtc))
}

func luaReadFile(name string, reader io.Reader) *luaFile {
	return &luaFile{name: name, reader: bufio.NewReader(reader)}
}

func luaWriteFile(name string, writer io.Writer) *luaFile {
	return &luaFile{name: name, writer: writer}
}

func (b *luaIOBridge) fileValue(file *luaFile) luart.Value {
	table := luart.NewTable()
	b.files[table] = file
	b.addFunc(table, "read", b.fileReadFunc(file, table), 1, true)
	b.addFunc(table, "write", b.fileWriteFunc(file, table), 1, true)
	b.addFunc(table, "lines", b.fileLinesFunc(file, table), 1, true)
	b.addFunc(table, "close", b.fileCloseFunc(file), 1, false)
	b.addFunc(table, "flush", b.fileFlushFunc(file), 1, false)
	return luart.TableValue(table)
}

func (b *luaIOBridge) openFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := c.Check1Arg(); err != nil {
			return nil, fmt.Errorf("check io.open arguments: %w", err)
		}
		name, err := c.StringArg(0)
		if err != nil {
			return nil, fmt.Errorf("read io.open path argument: %w", err)
		}
		mode := "r"
		if c.NArgs() >= 2 && !c.Arg(1).IsNil() {
			mode, err = c.StringArg(1)
			if err != nil {
				return nil, fmt.Errorf("read io.open mode argument: %w", err)
			}
		}
		file, err := b.openFile(name, mode)
		if err != nil {
			return luaPushIOError(t.Runtime, c, err), nil
		}
		return c.PushingNext1(t.Runtime, b.fileValue(file)), nil
	}
}

func (b *luaIOBridge) linesFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if c.NArgs() == 0 || c.Arg(0).IsNil() {
			return c.PushingNext1(t.Runtime, luart.FunctionValue(b.lineIterator(b.defaultInput, false))), nil
		}
		name, err := c.StringArg(0)
		if err != nil {
			return nil, fmt.Errorf("read io.lines path argument: %w", err)
		}
		file, err := b.openFile(name, "r")
		if err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.FunctionValue(b.lineIterator(file, true))), nil
	}
}

func (b *luaIOBridge) readFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		values, err := b.defaultInput.read(c.Etc())
		if err != nil {
			return nil, err
		}
		return c.PushingNext(t.Runtime, values...), nil
	}
}

func (b *luaIOBridge) writeFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := b.defaultOutput.write(c.Etc()); err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.BoolValue(true)), nil
	}
}

func (b *luaIOBridge) flushFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := b.defaultOutput.flush(); err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.BoolValue(true)), nil
	}
}

func (b *luaIOBridge) inputFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if c.NArgs() == 0 || c.Arg(0).IsNil() {
			return c.PushingNext1(t.Runtime, b.fileValue(b.defaultInput)), nil
		}
		file, err := b.fileFromValueOrPath(c.Arg(0), "r")
		if err != nil {
			return nil, err
		}
		b.defaultInput = file
		return c.PushingNext1(t.Runtime, b.fileValue(file)), nil
	}
}

func (b *luaIOBridge) outputFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if c.NArgs() == 0 || c.Arg(0).IsNil() {
			return c.PushingNext1(t.Runtime, b.fileValue(b.defaultOutput)), nil
		}
		file, err := b.fileFromValueOrPath(c.Arg(0), "w")
		if err != nil {
			return nil, err
		}
		b.defaultOutput = file
		return c.PushingNext1(t.Runtime, b.fileValue(file)), nil
	}
}

func (b *luaIOBridge) typeFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if c.NArgs() == 0 {
			return c.PushingNext1(t.Runtime, luart.NilValue), nil
		}
		file, ok := b.fileFromValue(c.Arg(0))
		if !ok {
			return c.PushingNext1(t.Runtime, luart.NilValue), nil
		}
		if file.closed {
			return c.PushingNext1(t.Runtime, luart.StringValue("closed file")), nil
		}
		return c.PushingNext1(t.Runtime, luart.StringValue("file")), nil
	}
}

func (b *luaIOBridge) fileReadFunc(file *luaFile, self *luart.Table) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		values, err := file.read(luaMethodArgs(c, self))
		if err != nil {
			return nil, err
		}
		return c.PushingNext(t.Runtime, values...), nil
	}
}

func (b *luaIOBridge) fileWriteFunc(file *luaFile, self *luart.Table) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := file.write(luaMethodArgs(c, self)); err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.BoolValue(true)), nil
	}
}

func (b *luaIOBridge) fileLinesFunc(file *luaFile, self *luart.Table) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		_ = luaMethodArgs(c, self)
		return c.PushingNext1(t.Runtime, luart.FunctionValue(b.lineIterator(file, false))), nil
	}
}

func (b *luaIOBridge) fileCloseFunc(file *luaFile) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := file.close(); err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.BoolValue(true)), nil
	}
}

func (b *luaIOBridge) fileFlushFunc(file *luaFile) luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := file.flush(); err != nil {
			return nil, err
		}
		return c.PushingNext1(t.Runtime, luart.BoolValue(true)), nil
	}
}

func (b *luaIOBridge) lineIterator(file *luaFile, closeAtEOF bool) *luart.GoFunction {
	fn := luart.NewGoFunction(func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		value, err := file.readLine(false)
		if err != nil {
			return nil, err
		}
		if value.IsNil() && closeAtEOF {
			if err := file.close(); err != nil {
				return nil, fmt.Errorf("close lua lines file: %w", err)
			}
		}
		return c.PushingNext1(t.Runtime, value), nil
	}, "invowk io lines iterator", 0, false)
	fn.SolemnlyDeclareCompliance(luart.ComplyCpuSafe | luart.ComplyMemSafe | luart.ComplyIoSafe)
	return fn
}

func (b *luaIOBridge) openFile(path, mode string) (*luaFile, error) {
	flag, readable, writable, err := luaOpenFlags(mode)
	if err != nil {
		return nil, err
	}
	normalized, err := b.pathValidator.validate(b.workDir, path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file, err := os.OpenFile(normalized, flag, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open lua file %q: %w", normalized, err)
	}
	luaFile := &luaFile{name: normalized, file: file, closable: true}
	if readable {
		luaFile.reader = bufio.NewReader(file)
	}
	if writable {
		luaFile.writer = file
	}
	return luaFile, nil
}

func luaOpenFlags(mode string) (flag int, readable, writable bool, err error) {
	switch strings.TrimSuffix(mode, "b") {
	case "r":
		return os.O_RDONLY, true, false, nil
	case "w":
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC, false, true, nil
	case "a":
		return os.O_WRONLY | os.O_CREATE | os.O_APPEND, false, true, nil
	case "r+":
		return os.O_RDWR, true, true, nil
	case "w+":
		return os.O_RDWR | os.O_CREATE | os.O_TRUNC, true, true, nil
	case "a+":
		return os.O_RDWR | os.O_CREATE | os.O_APPEND, true, true, nil
	default:
		return 0, false, false, fmt.Errorf("invalid io.open mode %q", mode)
	}
}

func (b *luaIOBridge) fileFromValueOrPath(value luart.Value, mode string) (*luaFile, error) {
	if file, ok := b.fileFromValue(value); ok {
		return file, nil
	}
	path, ok := value.TryString()
	if !ok {
		return nil, errors.New("expected file or filename")
	}
	return b.openFile(path, mode)
}

func (b *luaIOBridge) fileFromValue(value luart.Value) (*luaFile, bool) {
	table, ok := value.TryTable()
	if !ok {
		return nil, false
	}
	file, ok := b.files[table]
	return file, ok
}

func (f *luaFile) read(formats []luart.Value) ([]luart.Value, error) {
	if len(formats) == 0 {
		value, err := f.readLine(false)
		return []luart.Value{value}, err
	}
	values := make([]luart.Value, 0, len(formats))
	for _, format := range formats {
		value, err := f.readOne(format)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (f *luaFile) readOne(format luart.Value) (luart.Value, error) {
	if format.IsNil() {
		return f.readLine(false)
	}
	if count, ok := luart.ToInt(format); ok {
		return f.readBytes(count)
	}
	raw, ok := format.TryString()
	if !ok {
		return luart.NilValue, errors.New("file read format must be a string or integer")
	}
	switch raw {
	case "*a", "a":
		return f.readAll()
	case "*l", "l":
		return f.readLine(false)
	case "*L", "L":
		return f.readLine(true)
	default:
		return luart.NilValue, fmt.Errorf("unsupported file read format %q", raw)
	}
}

func (f *luaFile) readAll() (luart.Value, error) {
	if err := f.ensureReadable(); err != nil {
		return luart.NilValue, err
	}
	data, err := io.ReadAll(f.reader)
	if err != nil {
		return luart.NilValue, fmt.Errorf("read lua file: %w", err)
	}
	return luart.StringValue(string(data)), nil
}

func (f *luaFile) readLine(keepEnd bool) (luart.Value, error) {
	if err := f.ensureReadable(); err != nil {
		return luart.NilValue, err
	}
	line, err := f.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return luart.NilValue, fmt.Errorf("read lua file line: %w", err)
	}
	if line == "" && errors.Is(err, io.EOF) {
		return luart.NilValue, nil
	}
	if !keepEnd {
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
	}
	return luart.StringValue(line), nil
}

func (f *luaFile) readBytes(count int64) (luart.Value, error) {
	if count < 0 {
		return luart.NilValue, errors.New("byte count must not be negative")
	}
	if err := f.ensureReadable(); err != nil {
		return luart.NilValue, err
	}
	if count == 0 {
		_, err := f.reader.Peek(1)
		if errors.Is(err, io.EOF) {
			return luart.NilValue, nil
		}
		if err != nil {
			return luart.NilValue, fmt.Errorf("peek lua file: %w", err)
		}
		return luart.StringValue(""), nil
	}
	if count > int64(^uint(0)>>1) {
		return luart.NilValue, errors.New("byte count is too large")
	}
	buf := make([]byte, int(count))
	n, err := io.ReadFull(f.reader, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return luart.NilValue, fmt.Errorf("read lua file bytes: %w", err)
	}
	if n == 0 && errors.Is(err, io.EOF) {
		return luart.NilValue, nil
	}
	return luart.StringValue(string(buf[:n])), nil
}

func (f *luaFile) write(values []luart.Value) error {
	if err := f.ensureWritable(); err != nil {
		return err
	}
	for _, value := range values {
		text, ok := value.ToString()
		if !ok {
			return errors.New("file write arguments must be strings or numbers")
		}
		if _, err := io.WriteString(f.writer, text); err != nil {
			return fmt.Errorf("write lua file: %w", err)
		}
	}
	return nil
}

func (f *luaFile) flush() error {
	if f.closed {
		return errors.New(luaFileClosedMsg)
	}
	if f.file != nil {
		if err := f.file.Sync(); err != nil {
			return fmt.Errorf("sync lua file: %w", err)
		}
	}
	return nil
}

func (f *luaFile) close() error {
	if f.closed {
		return nil
	}
	if !f.closable {
		return errors.New("cannot close standard file")
	}
	f.closed = true
	if f.file != nil {
		if err := f.file.Close(); err != nil {
			return fmt.Errorf("close lua file: %w", err)
		}
	}
	return nil
}

func (f *luaFile) ensureReadable() error {
	if f.closed {
		return errors.New(luaFileClosedMsg)
	}
	if f.reader == nil {
		return errors.New("file is not open for reading")
	}
	return nil
}

func (f *luaFile) ensureWritable() error {
	if f.closed {
		return errors.New(luaFileClosedMsg)
	}
	if f.writer == nil {
		return errors.New("file is not open for writing")
	}
	return nil
}

func luaMethodArgs(c *luart.GoCont, self *luart.Table) []luart.Value {
	args := c.Etc()
	if c.NArgs() == 0 {
		return args
	}
	if table, ok := c.Arg(0).TryTable(); ok && table == self {
		return args
	}
	return append([]luart.Value{c.Arg(0)}, args...)
}

func luaPushIOError(r *luart.Runtime, c *luart.GoCont, err error) luart.Cont {
	return c.PushingNext(r, luart.NilValue, luart.StringValue(err.Error()))
}

func installLuaRequireBridge(r *luart.Runtime, scriptBasePath string) *luart.GoFunction {
	bridge := &luaRequireBridge{
		scriptBasePath: scriptBasePath,
		loaded:         make(map[string]luart.Value),
		loading:        make(map[string]bool),
	}
	return r.SetEnvGoFunc(r.GlobalEnv(), "require", bridge.requireFunc(), 1, false)
}

func (b *luaRequireBridge) requireFunc() luart.GoFunctionFunc {
	return func(t *luart.Thread, c *luart.GoCont) (luart.Cont, error) {
		if err := c.Check1Arg(); err != nil {
			return nil, fmt.Errorf("check require arguments: %w", err)
		}
		name, err := c.StringArg(0)
		if err != nil {
			return nil, fmt.Errorf("read require module argument: %w", err)
		}
		if value, ok := b.loaded[name]; ok {
			return c.PushingNext1(t.Runtime, value), nil
		}
		path, source, err := b.resolveModule(name)
		if err != nil {
			return nil, err
		}
		b.loaded[name] = luart.BoolValue(true)
		b.loading[name] = true
		defer delete(b.loading, name)

		chunk, err := t.CompileAndLoadLuaChunk(path, source, luart.TableValue(t.GlobalEnv()))
		if err != nil {
			delete(b.loaded, name)
			return nil, fmt.Errorf("compile required lua module %q: %w", name, err)
		}
		value, err := luart.Call1(t, luart.FunctionValue(chunk))
		if err != nil {
			delete(b.loaded, name)
			return nil, fmt.Errorf("run required lua module %q: %w", name, err)
		}
		if value.IsNil() {
			value = luart.BoolValue(true)
		}
		b.loaded[name] = value
		return c.PushingNext1(t.Runtime, value), nil
	}
}

func (b *luaRequireBridge) resolveModule(name string) (path string, source []byte, err error) {
	if validateErr := validateLuaRequireName(name); validateErr != nil {
		return "", nil, validateErr
	}
	if strings.TrimSpace(b.scriptBasePath) == "" {
		return "", nil, errors.New("lua require needs a script base path")
	}
	base, err := normalizeExistingOrParent(b.scriptBasePath, "")
	if err != nil {
		return "", nil, err
	}
	parts := make([]string, 0)
	for part := range strings.SplitSeq(name, ".") {
		parts = append(parts, part)
	}
	rel := filepath.FromSlash(strings.Join(parts, "/"))
	candidates := []string{
		filepath.Join(base, rel+".lua"),
		filepath.Join(base, rel, "init.lua"),
	}
	var tried []string
	for _, candidate := range candidates {
		normalized, normErr := normalizeExistingOrParent(candidate, base)
		if normErr != nil {
			return "", nil, normErr
		}
		if !pathWithin(base, normalized) {
			return "", nil, fmt.Errorf("lua require %q escapes script base path", name)
		}
		tried = append(tried, normalized)
		source, readErr := os.ReadFile(normalized)
		if readErr == nil {
			return normalized, source, nil
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			return "", nil, fmt.Errorf("read lua module %q: %w", normalized, readErr)
		}
	}
	return "", nil, fmt.Errorf("lua module %q not found; tried %s", name, strings.Join(tried, ", "))
}

func validateLuaRequireName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("lua module name must not be empty")
	}
	if strings.ContainsAny(name, `/\:`) {
		return fmt.Errorf("invalid lua module name %q", name)
	}
	for part := range strings.SplitSeq(name, ".") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid lua module name %q", name)
		}
	}
	return nil
}
