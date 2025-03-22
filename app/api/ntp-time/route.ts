import { NextRequest, NextResponse } from 'next/server';
import ntpClient from 'ntp-client';

export async function GET(req: NextRequest) {
  return new Promise((resolve) => {
    ntpClient.getNetworkTime("ntp.aliyun.com", 123, (err, date) => {
      if (err) {
        console.error("NTP 时间同步失败:", err);
        resolve(NextResponse.json({ error: "NTP 时间同步失败" }, { status: 500 }));
        return;
      }
      resolve(NextResponse.json({ timestamp: date.getTime() }));
    });
  });
}
