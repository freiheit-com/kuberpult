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

import { ConfirmationDialog, ConfirmationDialogProps, PlainDialog, PlainDialogProps } from './ConfirmationDialog';
import { act, getByTestId, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

const getNodeConfirm = (overrides?: {}): JSX.Element | any => {
    const props: any = {
        children: null,
        open: true,
        ...overrides,
    };
    // given
    return (
        <div>
            <MemoryRouter>
                <ConfirmationDialog {...props} />
            </MemoryRouter>
        </div>
    );
};

const getNodePlain = (overrides?: {}): JSX.Element | any => {
    const props: any = {
        open: true,
        center: false,
        ...overrides,
    };
    // given
    return (
        <div>
            <MemoryRouter>
                <PlainDialog {...props} />
            </MemoryRouter>
        </div>
    );
};

const addSibling = (wrappee: JSX.Element) => (
    <>
        {wrappee}
        <div data-testid="sibling-outside"></div>
    </>
);

const getWrapperConfirm = (overrides?: Partial<ConfirmationDialogProps>) =>
    render(addSibling(getNodeConfirm(overrides)));
const getWrapperPlain = (overrides?: Partial<PlainDialogProps>) => render(addSibling(getNodePlain(overrides)));

const testIdRootRefParent = 'test-root-ref-parent';

describe('ConfirmDialog', () => {
    type TestData = {
        name: string;
        props: Partial<ConfirmationDialogProps>;
        // if we click these elements
        clickViaEvent: string[];
        // click button
        clickButton: string[];
        sendKey: string;
        // then we expect dialog to be
        expectClose: boolean;
        expectRootRefClasses: string[];
    };

    type propMod = (props: Partial<ConfirmationDialogProps>) => void;
    const fixtureProps = (...mods: propMod[]) => {
        const props = {
            testIdRootRefParent: testIdRootRefParent,
            headerLabel: 'Header label',
            confirmLabel: 'Confirm label',
        };
        mods.forEach((m: propMod) => m(props));
        return props;
    };
    const fixtureTestData = () => ({
        props: fixtureProps(),
        clickViaEvent: [],
        clickButton: [],
        sendKey: '',
        expectClose: false,
        expectRootRefClasses: ['confirmation-dialog-container', 'confirmation-dialog-container-open'],
    });

    const data: TestData[] = [
        {
            name: 'visible if not clicked',
            ...fixtureTestData(),
        },
        {
            name: 'close if clicked outside',
            ...fixtureTestData(),
            clickViaEvent: ['sibling-outside'],
            expectClose: true,
        },
        {
            name: 'close if escape pressed',
            ...fixtureTestData(),
            clickViaEvent: ['sibling-outside'],
            sendKey: 'Escape',
            expectClose: true,
        },
        {
            name: 'close if cancel button pressed',
            ...fixtureTestData(),
            clickButton: ['test-confirm-button-cancel'],
            expectClose: true,
        },
        {
            name: 'do not close on random key press',
            ...fixtureTestData(),
            sendKey: 'a',
            expectClose: false,
        },
    ];

    const mouseClickEvents = ['pointerdown', 'mousedown', 'click', 'mouseup', 'pointerup'];

    describe.each(data)('Closes confirm dialog on user interaction', (testcase) => {
        it(testcase.name, () => {
            let calledClose = false;
            const onClose = () => {
                calledClose = true;
            };
            const { container } = getWrapperConfirm({
                ...testcase.props,
                onCancel: onClose,
                children: (
                    <>
                        <div data-testid="test-content"></div>
                    </>
                ),
            });

            testcase.clickViaEvent.forEach((id) => {
                const elem = getByTestId(container, id);
                mouseClickEvents.forEach((mouseEventType) => {
                    act(() =>
                        elem.dispatchEvent(
                            new MouseEvent(mouseEventType, {
                                view: window,
                                bubbles: true,
                                // cancelable: true,
                                buttons: 1,
                            })
                        )
                    );
                });
            });
            testcase.clickButton.forEach((id) => {
                act(() => getByTestId(container, id).click());
            });

            if (testcase.sendKey !== '') {
                ['keydown', 'keypress', 'keyup'].forEach((keyEventType) =>
                    act(() =>
                        document.dispatchEvent(
                            new KeyboardEvent(keyEventType, { key: testcase.sendKey, bubbles: true })
                        )
                    )
                );
            }

            expect(calledClose).toBe(testcase.expectClose);

            if (!testcase.expectClose) {
                const header = container.getElementsByClassName('confirmation-dialog-header');
                expect(header).toHaveLength(1);
                expect(header[0]).toHaveTextContent(testcase.props.headerLabel ?? '');

                const confirm = getByTestId(container, 'test-confirm-button-confirm');
                expect(confirm).toHaveTextContent(testcase.props.confirmLabel ?? '');
                expect(getByTestId(container, 'test-confirm-button-cancel')).toBeVisible();

                testcase.expectRootRefClasses.forEach((clas) =>
                    expect(getByTestId(container, testIdRootRefParent)).toHaveClass(clas)
                );
            }
        });
    });
});

describe('PlainDialog Closing', () => {
    type TestData = {
        name: string;
        props: Partial<PlainDialogProps>;
        // if we click these elements
        clickViaEvent: string[];
        sendKey: string;
        // then we expect dialog to be
        expectClose: boolean;
        expectRootRefClasses: string[];
    };

    type propMod = (props: Partial<PlainDialogProps>) => void;
    const fixtureProps = (...mods: propMod[]) => {
        const props = {
            testIdRootRefParent: testIdRootRefParent,
            disableBackground: false,
            center: false,
            children: (
                <>
                    <div data-testid="test-content-inside"></div>
                </>
            ),
        };
        mods.forEach((m: propMod) => m(props));
        return props;
    };
    const fixtureTestData = () => ({
        props: fixtureProps(),
        clickViaEvent: [],
        sendKey: '',
        expectClose: false,
        expectRootRefClasses: ['plain-dialog-container'],
    });

    const data: TestData[] = [
        {
            name: 'visible if not clicked',
            ...fixtureTestData(),
        },
        {
            name: 'close if clicked outside',
            ...fixtureTestData(),
            clickViaEvent: ['sibling-outside'],
            expectClose: true,
        },
        {
            name: 'still visible if clicked on content',
            ...fixtureTestData(),
            clickViaEvent: ['content-inside'],
            expectClose: false,
        },
        {
            name: 'still visible if clicked on content while centered',
            ...fixtureTestData(),
            props: fixtureProps((props) => {
                props.center = true;
            }),
            clickViaEvent: ['content-inside'],
            expectClose: false,
            expectRootRefClasses: ['confirmation-dialog-container'],
        },
        {
            name: 'still visible if clicked on content while centered with disabled background',
            ...fixtureTestData(),
            props: fixtureProps((props) => {
                props.center = true;
                props.disableBackground = true;
            }),
            clickViaEvent: ['content-inside'],
            expectClose: false,
            expectRootRefClasses: ['confirmation-dialog-container', 'confirmation-dialog-container-open'],
        },
        {
            name: 'close if clicked outside while centered',
            ...fixtureTestData(),
            props: fixtureProps((props) => {
                props.center = true;
            }),
            clickViaEvent: ['sibling-outside'],
            expectClose: true,
        },
        {
            name: 'different dialog class if centered',
            ...fixtureTestData(),
            props: fixtureProps((props) => {
                props.center = true;
            }),
            expectClose: false,
            expectRootRefClasses: ['confirmation-dialog-container'],
        },
        {
            name: 'close if escape pressed',
            ...fixtureTestData(),
            clickViaEvent: ['sibling-outside'],
            sendKey: 'Escape',
            expectClose: true,
        },
        {
            name: 'do not close on random key press',
            ...fixtureTestData(),
            sendKey: 'a',
            expectClose: false,
        },
    ];

    const mouseClickEvents = ['pointerdown', 'mousedown', 'click', 'mouseup', 'pointerup'];

    describe.each(data)('Closes plain dialog on user interaction', (testcase) => {
        it(testcase.name, () => {
            let calledClose = false;
            const onClose = () => {
                calledClose = true;
            };
            const { container } = getWrapperPlain({
                ...testcase.props,
                onClose: onClose,
                children: (
                    <>
                        <div data-testid="content-inside"></div>
                    </>
                ),
            });

            testcase.clickViaEvent.forEach((id) => {
                const elem = getByTestId(container, id);
                mouseClickEvents.forEach((mouseEventType) => {
                    elem.dispatchEvent(
                        new MouseEvent(mouseEventType, {
                            view: window,
                            bubbles: true,
                            buttons: 1,
                        })
                    );
                });
            });

            if (testcase.sendKey !== '') {
                ['keydown', 'keypress', 'keyup'].forEach((keyEventType) =>
                    document.dispatchEvent(new KeyboardEvent(keyEventType, { key: testcase.sendKey, bubbles: true }))
                );
            }

            expect(calledClose).toBe(testcase.expectClose);

            if (!testcase.expectClose) {
                testcase.expectRootRefClasses.forEach((clas) =>
                    expect(getByTestId(container, testIdRootRefParent)).toHaveClass(clas)
                );
            }
        });
    });
});
