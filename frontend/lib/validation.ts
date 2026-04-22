import { z } from "zod";

export const agentDidSchema = z
  .string()
  .trim()
  .min(6, "Agent DID 不能为空")
  .startsWith("did:", "Agent DID 必须以 did: 开头");

export const amountSchema = z
  .string()
  .trim()
  .regex(/^\d+(\.\d+)?$/, "金额格式错误")
  .refine((v) => Number(v) > 0, "金额必须大于 0");

export const authorizeSchema = z.object({
  agentDid: agentDidSchema,
  singleLimit: amountSchema,
  dailyLimit: amountSchema,
  whitelist: z.array(z.string().trim().min(1)).min(1, "至少一个白名单商户"),
});

export const paySchema = z.object({
  payerDid: agentDidSchema,
  merchantId: z.string().trim().min(1, "商户 ID 必填"),
  amount: amountSchema,
});

export const rechargeSchema = z.object({
  vaAccountId: z.string().trim().min(1, "VA 账户必填"),
  amount: amountSchema,
});
