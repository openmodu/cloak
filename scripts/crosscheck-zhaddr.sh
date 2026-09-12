#!/usr/bin/env bash
# 把中文地址融合的输出与上游 aifw 的 Zig 实现逐行对拍。
#
# 做法：单独编译上游的 core/merge_zh_addr.zig（它只依赖 recog_entity.zig，
# 不需要 Rust 正则库），用同一份数据集、同一种种子跑一遍，再和本仓库的金样本 diff。
#
# 用法：
#   scripts/crosscheck-zhaddr.sh [上游 aifw 仓库路径]
# 需要 zig 0.15.x 在 PATH 里，或用 ZIG 环境变量指定。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AIFW="${1:-$REPO_ROOT/../aifw}"
ZIG="${ZIG:-$(command -v zig || true)}"

if [[ -z "$ZIG" ]]; then
  echo "找不到 zig，请装好 zig 0.15.x 或设置 ZIG=/path/to/zig" >&2
  exit 1
fi
for f in core/merge_zh_addr.zig core/recog_entity.zig; do
  if [[ ! -f "$AIFW/$f" ]]; then
    echo "在 $AIFW 下找不到 $f，请把上游 aifw 仓库路径作为第一个参数传进来" >&2
    exit 1
  fi
done

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp "$AIFW/core/merge_zh_addr.zig" "$AIFW/core/recog_entity.zig" "$WORK/"

cat > "$WORK/dump.zig" <<'ZIG'
const std = @import("std");
const merge = @import("merge_zh_addr.zig");
const entity = @import("recog_entity.zig");

pub const std_options: std.Options = .{ .log_level = .warn };

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const alloc = gpa.allocator();

    const args = try std.process.argsAlloc(alloc);
    defer std.process.argsFree(alloc, args);

    const data = try std.fs.cwd().readFileAlloc(alloc, args[1], 1 << 22);
    defer alloc.free(data);

    var it = std.mem.splitScalar(u8, data, '\n');
    while (it.next()) |line| {
        if (line.len == 0) continue;
        const seeds = [_]entity.RecogEntity{.{
            .entity_type = .PHYSICAL_ADDRESS,
            .start = 0,
            .end = @intCast(line.len),
            .score = 0.9,
            .description = null,
        }};
        const res = try merge.mergeZhAddressSpans(alloc, line, seeds[0..]);
        defer alloc.free(res);
        if (res.len == 0) {
            std.debug.print("none\n", .{});
            continue;
        }
        for (res) |r| {
            std.debug.print("{d:.4}\t{d}\t{d}\t{s}\n", .{ r.score, r.start, r.end, line[r.start..r.end] });
        }
    }
}
ZIG

echo "编译上游实现..."
(cd "$WORK" && "$ZIG" build-exe dump.zig -O ReleaseSafe >/dev/null)

echo "跑数据集..."
"$WORK/dump" "$REPO_ROOT/testdata/zh_address_dataset.txt" 2> "$WORK/upstream.txt"

echo "对拍..."
if diff -u "$WORK/upstream.txt" "$REPO_ROOT/testdata/zh_address.golden.txt"; then
  echo "✓ 与上游 Zig 实现逐行一致"
else
  echo "✗ 存在差异，见上面的 diff（左：上游 Zig，右：本仓库金样本）" >&2
  exit 1
fi
