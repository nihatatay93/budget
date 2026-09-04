import { describe, expect, it } from "vitest";

import { categoryName, t } from "./i18n";

describe("category localization", () => {
  it("resolves predefined category keys in English and Turkish", () => {
    const category = { name: "groceries", kind: "expense" as const, predefined_key: "groceries" };
    expect(categoryName(category, "en")).toBe("Groceries");
    expect(categoryName(category, "tr")).toBe("Market");
    expect(t("category.income.salary", "tr")).toBe("Maaş");
  });

  it("keeps custom category names unchanged", () => {
    expect(categoryName({ name: "Evcil hayvan", kind: "expense" }, "en")).toBe("Evcil hayvan");
  });

  it("uses Turkish translations for shared web UI labels and interpolated text", () => {
    expect(t("Transactions", "tr")).toBe("İşlemler");
    expect(t("Add transaction", "tr")).toBe("İşlem ekle");
    expect(t("{shown} of {total}", { shown: 2, total: 5 }, "tr")).toBe("2 / 5");
    expect(t("account.type.credit_card", "tr")).toBe("Kredi kartı");
    expect(t("Category appearance", "tr")).toBe("Kategori görünümü");
    expect(t("category.icon.shopping-cart", "tr")).toBe("Market");
    expect(t("category.color.purple", "tr")).toBe("Mor");
  });
});
