package settings

import (
	"reflect"
	"testing"
)

// nonzero 递归构造该类型的非零值。mergeFrom 的语义是"非零字段才覆盖"，
// 因此用全非零的源对象合并进零值目标后，两侧必须完全相等——任何
// Settings 新增字段若忘了在 mergeFrom 里处理，本测试即失败
// （历史上 DNS 字段就漏过一次，重启后用户配置静默丢失）。
func nonzero(t *testing.T, typ reflect.Type, depth int) reflect.Value {
	t.Helper()
	if depth > 6 {
		t.Fatalf("类型递归过深: %s", typ)
	}
	switch typ.Kind() {
	case reflect.String:
		return reflect.ValueOf("v").Convert(typ)
	case reflect.Bool:
		return reflect.ValueOf(true).Convert(typ)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(1).Convert(typ)
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(1.5).Convert(typ)
	case reflect.Slice:
		sl := reflect.MakeSlice(typ, 1, 1)
		sl.Index(0).Set(nonzero(t, typ.Elem(), depth+1))
		return sl
	case reflect.Map:
		m := reflect.MakeMapWithSize(typ, 1)
		m.SetMapIndex(nonzero(t, typ.Key(), depth+1), nonzero(t, typ.Elem(), depth+1))
		return m
	case reflect.Pointer:
		p := reflect.New(typ.Elem())
		p.Elem().Set(nonzero(t, typ.Elem(), depth+1))
		return p
	case reflect.Struct:
		v := reflect.New(typ).Elem()
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			v.Field(i).Set(nonzero(t, f.Type, depth+1))
		}
		return v
	default:
		t.Fatalf("Settings 出现守卫测试未覆盖的字段类型 %s（%s），请同步更新本测试", typ, typ.Kind())
		return reflect.Value{}
	}
}

// TestMergeFromCoversEveryField 固化契约：mergeFrom 之后目标必须与
// 全非零源完全一致；失败信息直接点名漏合并的字段。
func TestMergeFromCoversEveryField(t *testing.T) {
	o := nonzero(t, reflect.TypeOf(Settings{}), 0).Interface().(Settings)
	s := Settings{}
	s.mergeFrom(&o)
	if reflect.DeepEqual(s, o) {
		return
	}
	sv, ov := reflect.ValueOf(s), reflect.ValueOf(o)
	st := reflect.TypeOf(s)
	for i := 0; i < sv.NumField(); i++ {
		if !reflect.DeepEqual(sv.Field(i).Interface(), ov.Field(i).Interface()) {
			t.Errorf("字段 %s（json:%s）未被 mergeFrom 合并：got %#v want %#v",
				st.Field(i).Name, st.Field(i).Tag.Get("json"),
				sv.Field(i).Interface(), ov.Field(i).Interface())
		}
	}
}
