'use client';

import Image from 'next/image'
import Link from 'next/link'
import { useEffect, useState } from 'react';

export default function Home() {
  const [currentTime, setCurrentTime] = useState('00:00:00');
  const [examInfo, setExamInfo] = useState('');

  useEffect(() => {
    async function loadExams() {
      try {
        const response = await fetch('/api/exams');
        const exams = await response.json();
        return exams;
      } catch (error) {
        console.error('加载考试数据失败', error);
        return [];
      }
    }

    function formatTime(ms: number) {
      const totalSeconds = Math.floor(ms / 1000);
      const hours = Math.floor(totalSeconds / 3600);
      const minutes = Math.floor((totalSeconds % 3600) / 60);
      const seconds = totalSeconds % 60;
      return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    }

    async function updateTime() {
      const exams = await loadExams();
      
      function refresh() {
        const now = new Date();
        setCurrentTime(now.toLocaleTimeString('zh-CN', { hour12: false }).padStart(8, '0'));

        const nextExam = exams.find((exam: any) => {
          const endTime = new Date(new Date(exam.start_at).getTime() + exam.duration_hour * 3600000);
          return now < endTime;
        });

        if (!nextExam) {
          setExamInfo('');
          return;
        }

        const startTime = new Date(nextExam.start_at).getTime();
        const endTime = startTime + nextExam.duration_hour * 3600000;
        
        if (now.getTime() < startTime) {
          setExamInfo(
            `${nextExam.name} | 考试未开始 | 剩余 ${formatTime(startTime - now.getTime())}`
          );
        } else if (now.getTime() >= startTime && now.getTime() < endTime) {
          setExamInfo(
            `${nextExam.name} | 考试进行中 | 剩余 ${formatTime(endTime - now.getTime())}`
          );
        }
        
        
      }

      refresh();
      setInterval(refresh, 1000);
    }
    
    updateTime();
  }, []);

  return (
    <main className="flex min-h-screen flex-col items-center justify-between p-24">
      <div className="z-10 w-full max-w-5xl items-center justify-between font-mono text-sm lg:flex">
        <p className="fixed left-0 top-0 flex w-full justify-center border-b border-gray-300 bg-gradient-to-b from-zinc-200 pb-6 pt-8 backdrop-blur-2xl dark:border-neutral-800 dark:bg-zinc-800/30 dark:from-inherit lg:static lg:w-auto  lg:rounded-xl lg:border lg:bg-gray-200 lg:p-4 lg:dark:bg-zinc-800/30">
          Sponsored by&nbsp;
          <Link href="https://met6.top/" target='_blank' rel='noopener noreferrer'>
            <code className="font-mono font-bold">Met6.top</code>
          </Link>
        </p>
        <div className="fixed bottom-0 left-0 flex h-48 w-full items-end justify-center bg-gradient-to-t from-white via-white dark:from-black dark:via-black lg:static lg:h-auto lg:w-auto lg:bg-none">
          <a
            className="pointer-events-none flex place-items-center gap-2 p-8 lg:pointer-events-auto lg:p-0"
            href="https://github.com/MeTerminator/met-box"
            target="_blank"
            rel="noopener noreferrer"
          >
            By{' '}
            <Image
              src="/vercel.svg"
              alt="Vercel Logo"
              className="dark:invert"
              width={100}
              height={24}
              priority
            />
          </a>
        </div>
      </div>

      <div className="relative flex flex-col items-center place-items-center 
            before:absolute before:h-[300px] before:w-[480px] before:-translate-x-1/2 before:rounded-full 
            before:bg-gradient-radial before:from-white before:to-transparent before:blur-2xl before:content-[''] 
            after:absolute after:-z-20 after:h-[180px] after:w-[240px] after:translate-x-1/3 
            after:bg-gradient-conic after:from-sky-200 after:via-blue-200 after:blur-2xl after:content-[''] 
            before:dark:bg-gradient-to-br before:dark:from-transparent before:dark:to-blue-700 before:dark:opacity-10 
            after:dark:from-sky-900 after:dark:via-[#0141ff] after:dark:opacity-40 before:lg:h-[360px]">
          <p className="time-display text-9xl font-bold mb-5">{currentTime}</p>
          <p className="info text-2xl">
            {examInfo.split('|').map((text, index) => (
              <span key={index}>
                {text}
                {index < examInfo.split('|').length - 1 && <span className="text-gray-500 mx-2">|</span>}
              </span>
            ))}
          </p>

      </div>
      <div className="mb-32 grid text-center lg:mb-0 lg:grid-cols-4 lg:text-left"></div>
    </main>
  )
}
