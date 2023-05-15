/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/
import React from 'react';

export const getRelativeDate = (date: Date): string => {
    const millisecondsPerHour = 1000 * 60 * 60; // 1000ms * 60s * 60m
    const elapsedTime = Date.now().valueOf() - date.valueOf();
    const hoursSinceDate = Math.floor(elapsedTime / millisecondsPerHour);

    if (hoursSinceDate < 24) {
        // recent date, calculate relative time in hours
        if (hoursSinceDate === 0) {
            return '< 1 hour ago';
        } else if (hoursSinceDate === 1) {
            return '1 hour ago';
        } else {
            return `${hoursSinceDate} hours ago`;
        }
    } else {
        // too many hours, calculate relative time in days
        const daysSinceDate = Math.floor(hoursSinceDate / 24);
        if (daysSinceDate === 1) {
            return '1 day ago';
        } else {
            return `${daysSinceDate} days ago`;
        }
    }
};

export const FormattedDate: React.FC<{ createdAt: Date; className?: string }> = ({ createdAt, className }) => {
    // Adds leading zero to get two digit day and month
    const twoDigit = (num: number): string => (num < 10 ? '0' : '') + num;
    // date format (ISO): yyyy-mm-dd, with no leading zeros, month is 0-indexed.
    // createdAt.toISOString() can't be used because it ignores the current time zone.
    const formattedDate = `${createdAt.getFullYear()}-${twoDigit(createdAt.getMonth() + 1)}-${twoDigit(
        createdAt.getDate()
    )}`;

    // getHours automatically gets the hours in the correct timezone. in 24h format (no timezone calculation needed)
    const formattedTime = `${createdAt.getHours()}:${createdAt.getMinutes()}`;

    return (
        <div className={className} title={createdAt.toString()}>
            {formattedDate + ' @ ' + formattedTime + ' | '}
            <i>{getRelativeDate(createdAt)}</i>
        </div>
    );
};
