"use client";

import { ResponsiveTimeRange } from "@nivo/calendar";
import { useUsage } from "../hooks/useData";
import { DataState, useDataState } from "./DataState";
import Card from "./ui/Card";
import { useMemo } from "react";
import { UsageRow } from "../types";

export default function ActivityChart() {
    // Fetch all time data (approx 27 years)
    const swrResponse = useUsage(10000);
    const { data, error, isLoading, hasData } = useDataState(swrResponse);

    const title = (
        <div className="flex items-center gap-2">
            <span>Activity Chart</span>
        </div>
    );

    const calendarData = useMemo(() => {
        if (!data) return [];

        // Aggregate hours by day across all users
        const dailyUsage: Record<string, number> = {};
        data.forEach((row: UsageRow) => {
            // Ensure date format is YYYY-MM-DD
            const date = row.day.split("T")[0];
            dailyUsage[date] = (dailyUsage[date] || 0) + row.hours;
        });

        return Object.entries(dailyUsage).map(([day, value]) => ({
            day,
            value: Math.round(value * 10) / 10, // Round to 1 decimal
        }));
    }, [data]);

    const { fromDate, toDate } = useMemo(() => {
        const now = new Date();
        // Dynamic "Last 12 Months"
        const to = now.toISOString().split("T")[0];

        // Go back 1 year
        const fromDateObj = new Date(now);
        fromDateObj.setFullYear(now.getFullYear() - 1);
        const from = fromDateObj.toISOString().split("T")[0];

        return {
            fromDate: from,
            toDate: to
        };
    }, []);

    // Custom theme for Nivo to match dark mode and app contrast
    const theme = {
        text: {
            fill: "#ffffff",
            fontSize: 12,
        },
        tooltip: {
            container: {
                background: "#000000", // Pure black background
                color: "#ffffff", // White text
                fontSize: 12,
                borderRadius: 4,
                boxShadow: "0 4px 6px -1px rgba(255, 255, 255, 0.1)",
                border: "1px solid #333",
            },
        },
    };

    return (
        <DataState
            isLoading={isLoading}
            error={error}
            data={data}
            errorFallback={
                <Card title={title}>
                    <div className="text-red-300 text-sm">Unable to load activity data</div>
                    <div className="text-xs text-red-400 mt-1">Check server connection</div>
                </Card>
            }
            loadingFallback={
                <Card title={title}>
                    <div className="h-[280px] flex items-center justify-center animate-pulse">
                        <div className="text-neutral-600">Loading activity...</div>
                    </div>
                </Card>
            }
        >
            {hasData && (
                <Card title={title}>
                    <div className="h-[280px] w-full">
                        <ResponsiveTimeRange
                            data={calendarData}
                            from={fromDate}
                            to={toDate}
                            maxValue={10} // Cap scale at 10 hours to show more color variation for typical usage
                            emptyColor="#262626" // neutral-800
                            // Removed the first dark color so low usage is visible
                            colors={["#0e4429", "#006d32", "#26a641", "#39d353"]}
                            // Show all days of the week (0=Sunday to 6=Saturday)
                            weekdayTicks={[0, 1, 2, 3, 4, 5, 6]}
                            weekdays={["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]}

                            margin={{ top: 20, right: 50, bottom: 20, left: 60 }} // Reduced margin for short names
                            dayBorderWidth={2}
                            dayBorderColor="#171717" // neutral-900 (background color)
                            weekdayLegendOffset={0}
                            monthLegendOffset={10}
                            theme={theme}
                            tooltip={({ day, value }) => (
                                <div className="bg-gray-800 text-gray-100 p-3 rounded shadow-lg border border-gray-700 whitespace-nowrap min-w-[100px] text-center">
                                    <strong className="block mb-1">{day}</strong>
                                    <span>{value} hours</span>
                                </div>
                            )}
                        />
                    </div>
                    <div className="flex items-center justify-end gap-2 mt-2 text-xs text-gray-500">
                        <span>Less</span>
                        <div className="flex gap-1">
                            <div className="w-3 h-3 bg-[#0e4429] rounded-sm" />
                            <div className="w-3 h-3 bg-[#006d32] rounded-sm" />
                            <div className="w-3 h-3 bg-[#26a641] rounded-sm" />
                            <div className="w-3 h-3 bg-[#39d353] rounded-sm" />
                        </div>
                        <span>More</span>
                    </div>
                </Card>
            )}
        </DataState>
    );
}
