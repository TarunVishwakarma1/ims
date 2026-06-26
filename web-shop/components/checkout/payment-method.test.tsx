import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { PaymentMethod } from "./payment-method";

describe("PaymentMethod", () => {
  it("disables COD when over max", () => {
    render(
      <PaymentMethod
        options={[
          { id: "razorpay", enabled: true },
          { id: "cod", enabled: false, min_paise: 10000, max_paise: 500000, reason: "max_value_exceeded" },
        ]}
        value="razorpay"
        onChange={() => {}}
      />,
    );
    const cod = screen.getByRole("radio", { name: /Cash on Delivery/i });
    expect(cod).toBeDisabled();
    expect(screen.getByText(/exceeds.+5,000.+COD limit/i)).toBeInTheDocument();
  });

  it("disables COD when under min", () => {
    render(
      <PaymentMethod
        options={[
          { id: "razorpay", enabled: true },
          { id: "cod", enabled: false, min_paise: 10000, max_paise: 500000, reason: "min_value_below" },
        ]}
        value="razorpay"
        onChange={() => {}}
      />,
    );
    expect(screen.getByText(/below.+100.+minimum/i)).toBeInTheDocument();
  });
});
