import { NextRequest, NextResponse } from 'next/server';
import ntpClient from 'ntp-client';

export async function GET(req: NextRequest): Promise<NextResponse> {
  try {
    const date = await new Promise<Date>((resolve, reject) => {
      ntpClient.getNetworkTime("ntp.aliyun.com", 123, (err, date) => {
        if (err || !date) {
          reject(new Error("NTP 时间同步失败"));
        } else {
          resolve(date);
        }
      });
    });

    return NextResponse.json({ timestamp: date.getTime() });
  } catch (error) {
    console.error("NTP 时间同步失败:", error);
    return NextResponse.json({ error: "NTP 时间同步失败" }, { status: 500 });
  }
}
