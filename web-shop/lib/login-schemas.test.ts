import { describe, it, expect } from "vitest";
import { phoneSchema, codeSchema } from "./login-schemas";

describe("phoneSchema", () => {
  it("accepts plain 10-digit", () => {
    expect(phoneSchema.parse({ phone: "9876543210" }).phone).toBe("9876543210");
  });
  it("accepts +91 prefix", () => {
    expect(phoneSchema.parse({ phone: "+91 9876543210" }).phone).toBe("9876543210");
  });
  it("accepts 91 prefix with dash", () => {
    expect(phoneSchema.parse({ phone: "91-9876543210" }).phone).toBe("9876543210");
  });
  it("strips spaces", () => {
    expect(phoneSchema.parse({ phone: "987 654 3210" }).phone).toBe("9876543210");
  });
  it("rejects 9-digit", () => {
    expect(() => phoneSchema.parse({ phone: "123456789" })).toThrow();
  });
  it("rejects letters", () => {
    expect(() => phoneSchema.parse({ phone: "abcdefghij" })).toThrow();
  });
  it("rejects empty", () => {
    expect(() => phoneSchema.parse({ phone: "" })).toThrow();
  });
});

describe("codeSchema", () => {
  it("accepts 6 digits", () => {
    expect(codeSchema.parse({ code: "123456" }).code).toBe("123456");
  });
  it("rejects 5 digits", () => {
    expect(() => codeSchema.parse({ code: "12345" })).toThrow();
  });
  it("rejects letters", () => {
    expect(() => codeSchema.parse({ code: "abcdef" })).toThrow();
  });
  it("rejects empty", () => {
    expect(() => codeSchema.parse({ code: "" })).toThrow();
  });
});
