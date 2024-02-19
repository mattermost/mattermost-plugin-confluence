export const splitArgs = (command) => {
    const myRegexp = /[^\s"]+|"([^"]*)"/gi;
    const myArray = [];
    let match;
    do {
        match = myRegexp.exec(command);
        if (match != null) {
            myArray.push(match[1] ? match[1] : match[0]);
        }
    } while (match != null);
    return myArray;
};

export const encodeToBase64 = (baseURL) => {
    return Buffer.from(baseURL).toString('base64');
};
