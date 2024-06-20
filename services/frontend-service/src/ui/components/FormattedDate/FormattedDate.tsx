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

Copyright freiheit.com*/
import React, { useEffect } from 'react';

const getRelativeDate = (current: Date, target: Date | undefined): string => {
    if (!target) return '';
    const elapsedTime = current.valueOf() - target.valueOf();
    const msPerMinute = 1000 * 60;
    const msPerHour = msPerMinute * 60;
    const msPerDay = msPerHour * 24;
    const msPerMonth = msPerDay * 30;
    const msPerYear = msPerDay * 365;

    if (elapsedTime < msPerMinute) {
        return 'just now';
    } else if (elapsedTime < msPerHour) {
        return Math.round(elapsedTime / msPerMinute) === 1
            ? `1 minute ago`
            : `${Math.round(elapsedTime / msPerMinute)} minutes ago`;
    } else if (elapsedTime < msPerDay) {
        return Math.round(elapsedTime / msPerHour) === 1
            ? '1 hour ago'
            : `${Math.round(elapsedTime / msPerHour)} hours ago`;
    } else if (elapsedTime < msPerMonth) {
        return Math.round(elapsedTime / msPerDay) === 1
            ? '~ 1 day ago'
            : `~ ${Math.round(elapsedTime / msPerDay)} days ago`;
    } else if (elapsedTime < msPerYear) {
        return Math.round(elapsedTime / msPerMonth) === 1
            ? '~ 1 month ago'
            : `~ ${Math.round(elapsedTime / msPerMonth)} months ago`;
    } else {
        return Math.round(elapsedTime / msPerYear) === 1
            ? '~ 1 year ago'
            : `~ ${Math.round(elapsedTime / msPerYear)} years ago`;
    }
};

export const FormattedDate: React.FC<{ createdAt: Date; className?: string }> = ({ createdAt, className }) => {
    const [relativeDate, setRelativeDate] = React.useState(getRelativeDate(new Date(), createdAt));
    useEffect(() => {
        const handle = setInterval(() => setRelativeDate(getRelativeDate(new Date(), createdAt)), 20000);
        return () => clearInterval(handle);
    }, [createdAt]);

    return (
        <span className={className} title={createdAt.toString()}>
            <i>{relativeDate}</i>
        </span>
    );
};
