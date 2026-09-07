// The message-template half of the Wails seam: the template types and the calls that read, write and
// delete them, plus the one that fetches a template's files. It lives beside api.ts rather than inside
// it because api.ts is one of the modules over the size limit that the guard in src/test/loc.test.ts is
// ratcheting down; a cohesive group of calls with its own types is exactly the kind of thing that should
// leave it, as the filter rules already have (see apiRules). The api object spreads what is exported
// here, so callers still reach these through api.* and nothing else changes.
import {DeleteTemplate, ListTemplates, SaveTemplate, TemplateFiles} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

export type Template = main.TemplateDTO
// TemplateAttachment describes one file a template carries: a name, a media type and a size, with no
// content. The listing fills the compose picker, while a template may hold a whole message's worth of
// bytes, so the bytes travel only for the one template being inserted or edited.
export type TemplateAttachment = main.TemplateAttachmentDTO
// TemplateFile is one of those files WITH its bytes, base64 encoded, in the shape the compose window
// already holds a pasted file in. That is what lets an inserted template's files join a message through
// the path the composer has always used.
export type TemplateFile = main.TemplateFileDTO

// TemplateInput is the shape sent back to save a template; an empty id means a new one.
//
// keepFilePositions names the stored files to carry over, by their position on the stored template and
// in the order they should end up in; anything not named is dropped. addFilePaths are newly chosen
// files, which follow. A stored file is named rather than resent, so editing a subject does not push the
// template's bytes across the bridge and back.
export interface TemplateInput {
    id: string
    name: string
    subject: string
    body: string
    keepFilePositions: number[]
    addFilePaths: string[]
}

export const templatesApi = {
    listTemplates: (): Promise<Template[]> => ListTemplates(),
    saveTemplate: (req: TemplateInput): Promise<void> => SaveTemplate(main.TemplateRequest.createFrom(req)),
    deleteTemplate: (templateId: string): Promise<void> => DeleteTemplate(templateId),
    // templateFiles reads one template's attachments with their bytes, for inserting it into a message.
    templateFiles: (templateId: string): Promise<TemplateFile[]> => TemplateFiles(templateId),
}
