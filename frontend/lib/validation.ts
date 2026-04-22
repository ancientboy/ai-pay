import { z } from "zod";
import { Locale, t } from "@/lib/i18n";

export function getValidationSchemas(locale: Locale) {
  const agentDidSchema = z
    .string()
    .trim()
    .min(6, t(locale, "validation.agentDidEmpty"))
    .startsWith("did:", t(locale, "validation.agentDidFormat"));

  const amountSchema = z
    .string()
    .trim()
    .regex(/^\d+(\.\d+)?$/, t(locale, "validation.amountFormat"))
    .refine((v) => Number(v) > 0, t(locale, "validation.amountPositive"));

  const authorizeSchema = z.object({
    agentDid: agentDidSchema,
    singleLimit: amountSchema,
    dailyLimit: amountSchema,
    whitelist: z
      .array(z.string().trim().min(1))
      .min(1, t(locale, "validation.whitelistMin")),
  });

  const paySchema = z.object({
    payerDid: agentDidSchema,
    merchantId: z.string().trim().min(1, t(locale, "validation.merchantRequired")),
    amount: amountSchema,
  });

  const rechargeSchema = z.object({
    vaAccountId: z.string().trim().optional(),
    vaCardNo: z.string().trim().optional(),
    amount: amountSchema,
  }).refine(
    (input) => Boolean(input.vaAccountId?.trim() || input.vaCardNo?.trim()),
    {
      message: t(locale, "validation.rechargeTargetRequired"),
      path: ["vaAccountId"],
    },
  );

  return {
    agentDidSchema,
    amountSchema,
    authorizeSchema,
    paySchema,
    rechargeSchema,
  };
}
