import { describe, it, expect } from "vitest";
import { safeNext } from "./safe-next";

describe("safeNext", () => {
  it("returns '/' for null/undefined/empty", () => {
    expect(safeNext(null)).toBe("/");
    expect(safeNext(undefined)).toBe("/");
    expect(safeNext("")).toBe("/");
  });

  it("rejects absolute URLs", () => {
    expect(safeNext("https://evil.com")).toBe("/");
    expect(safeNext("http://evil.com/path")).toBe("/");
  });

  it("rejects scheme-relative URLs", () => {
    expect(safeNext("//evil.com")).toBe("/");
  });

  it("rejects backslash path tricks", () => {
    expect(safeNext("\\path")).toBe("/");
  });

  it("rejects javascript: pseudo-scheme", () => {
    expect(safeNext("javascript:alert(1)")).toBe("/");
  });

  it("rejects relative paths not starting with /", () => {
    expect(safeNext("orders")).toBe("/");
    expect(safeNext("./orders")).toBe("/");
  });

  it("accepts same-origin absolute paths", () => {
    expect(safeNext("/orders")).toBe("/orders");
    expect(safeNext("/cart")).toBe("/cart");
    expect(safeNext("/orders/abc-123")).toBe("/orders/abc-123");
  });
});
