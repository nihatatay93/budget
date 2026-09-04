import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CategoryAppearance, categoryColorStyle, isSingleGrapheme } from "./CategoryAppearance";

describe("CategoryAppearance", () => {
  it("maps a semantic system key through the shared icon registry", () => {
    const { container } = render(<CategoryAppearance colorKey="green" iconType="system" iconValue="shopping-cart" label="Groceries" />);
    expect(screen.getByRole("img", { name: "Groceries" })).toBeInTheDocument();
    expect(container.querySelector("svg")).toBeInTheDocument();
    expect(container.querySelector(".category-appearance-icon")).toHaveStyle("--category-accent: #287a54");
  });

  it("renders Unicode emoji clusters as one category icon", () => {
    render(<CategoryAppearance colorKey="purple" iconType="emoji" iconValue="👩🏽‍💻" label="Work" />);
    expect(screen.getByText("👩🏽‍💻")).toBeInTheDocument();
    expect(isSingleGrapheme("👩🏽‍💻")).toBe(true);
    expect(isSingleGrapheme("🍀🍲")).toBe(false);
  });

  it("uses the ellipsis fallback and slate palette for unknown values", () => {
    const { container } = render(<CategoryAppearance colorKey="unknown" iconType="system" iconValue="not-a-system-icon" label="Other" />);
    expect(container.querySelector("svg")).toBeInTheDocument();
    expect(categoryColorStyle("unknown")).toMatchObject({ "--category-accent": "#66776c" });
  });
});
