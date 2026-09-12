package placeholder

import "testing"

func TestRenderMatchesUpstreamFormat(t *testing.T) {
	cases := []struct {
		tag  string
		id   uint32
		want string
	}{
		{"EMAIL_ADDRESS", 1, "__PII_EMAIL_ADDRESS_1__"},
		{"PHONE_NUMBER", 42, "__PII_PHONE_NUMBER_42__"},
		// 序号直接十进制格式化，不做零填充
		{"URL_ADDRESS", 11, "__PII_URL_ADDRESS_11__"},
	}
	for _, c := range cases {
		if got := Render(c.tag, c.id); got != c.want {
			t.Fatalf("got %q, want %q", got, c.want)
		}
	}
}
