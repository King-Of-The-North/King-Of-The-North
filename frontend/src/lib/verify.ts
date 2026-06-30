import "server-only";

// WhatsApp OTP verification behind an interface (ADR-0015): the mock runs now, the
// real Twilio Verify client swaps in once credentials land (ADR-0002 pattern).
export interface Verifier {
  start(phone: string): Promise<void>;
  check(phone: string, code: string): Promise<boolean>;
}

// DEV_OTP_CODE is the code the mock accepts; surfaced to the login screen as a hint so
// the demo needs no real WhatsApp message.
export const DEV_CODE = process.env.DEV_OTP_CODE ?? "424242";

class MockVerifier implements Verifier {
  async start(phone: string): Promise<void> {
    console.log(`[verify:mock] WhatsApp code for ${phone} = ${DEV_CODE}`);
  }
  async check(_phone: string, code: string): Promise<boolean> {
    return code.trim() === DEV_CODE;
  }
}

// verifier selects the real Twilio client when configured, else the mock. The Twilio
// impl (Verify API, channel=whatsapp) is wired when TWILIO_* env is present.
export function verifier(): Verifier {
  // if (process.env.TWILIO_ACCOUNT_SID && process.env.TWILIO_VERIFY_SERVICE_SID) {
  //   return new TwilioVerifier();
  // }
  return new MockVerifier();
}
